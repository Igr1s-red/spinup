package engine

// Ubuntu cloud image resolver.
//
// Ubuntu publishes images with a stable "current" symlink:
//   https://cloud-images.ubuntu.com/<codename>/current/
//
// The filename template is fixed per codename:
//   <codename>-server-cloudimg-<arch>.img     (raw/qcow2 depending on series)
//
// SHA256SUMS lives in the same directory. We fetch it to get a verified
// checksum at pull time rather than shipping hardcoded hashes that go stale.

import (
	"bufio"
	"fmt"
	"strings"
)

// ubuntuGoarchMap maps Go arch names to the Ubuntu cloud image arch suffix.
var ubuntuGoarchMap = map[string]string{
	"amd64": "amd64",
	"arm64": "arm64",
}

// resolveUbuntu returns a Resolver for the named Ubuntu codename.
// Ubuntu images use ".img" extension (internally qcow2 for most series).
func resolveUbuntu(codename string) func(goarch string) (string, string, error) {
	return func(goarch string) (string, string, error) {
		uArch, ok := ubuntuGoarchMap[goarch]
		if !ok {
			return "", "", fmt.Errorf("ubuntu: unsupported architecture %q", goarch)
		}

		baseURL := fmt.Sprintf(
			"https://cloud-images.ubuntu.com/%s/current", codename)
		filename := fmt.Sprintf("%s-server-cloudimg-%s.img", codename, uArch)
		downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

		// Fetch SHA256SUMS to get the checksum for this exact build.
		sumsURL := fmt.Sprintf("%s/SHA256SUMS", baseURL)
		body, err := fetchText(sumsURL)
		if err != nil {
			// Non-fatal: proceed without checksum verification if the sums
			// file is temporarily unavailable.
			return downloadURL, "", nil
		}

		return downloadURL, parseUbuntuSHA256SUMS(body, filename), nil
	}
}

// parseUbuntuSHA256SUMS finds the checksum for filename in an Ubuntu SHA256SUMS
// body. Lines have the form "<hash> *<filename>". Returns "" if not found.
func parseUbuntuSHA256SUMS(body, filename string) string {
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		if strings.TrimLeft(parts[1], "*") == filename {
			return parts[0]
		}
	}
	return ""
}
