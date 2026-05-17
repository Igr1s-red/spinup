package engine

// Fedora cloud image resolver.
//
// Fedora does not publish a "latest" symlink for its cloud images. Instead
// we discover the current stable release number by listing the releases
// directory, pick the highest plain integer (skipping "test" and "rawhide"),
// then find the exact qcow2 filename inside the Cloud/<arch>/images/ path.
//
// Two HTTP requests:
//   1. GET https://dl.fedoraproject.org/pub/fedora/linux/releases/
//      → parse HTML directory listing for the highest version dir
//   2. GET https://dl.fedoraproject.org/pub/fedora/linux/releases/<ver>/Cloud/<arch>/images/
//      → parse HTML listing for the Cloud-Base-Generic qcow2

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// fedoraGoarchMap maps Go arch names to Fedora's directory / filename arch names.
var fedoraGoarchMap = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
}

// resolveFedora returns a Resolver that discovers the latest stable Fedora
// cloud image URL and checksum at pull time.
func resolveFedora() func(goarch string) (string, string, error) {
	return func(goarch string) (string, string, error) {
		fArch, ok := fedoraGoarchMap[goarch]
		if !ok {
			return "", "", fmt.Errorf("fedora: unsupported architecture %q", goarch)
		}

		// ── Step 1: find latest stable release number ─────────────────────────
		releasesURL := "https://dl.fedoraproject.org/pub/fedora/linux/releases/"
		body, err := fetchText(releasesURL)
		if err != nil {
			return "", "", fmt.Errorf("fedora: listing releases: %w", err)
		}

		version, err := parseFedoraLatestVersion(body)
		if err != nil {
			return "", "", fmt.Errorf("fedora: %w", err)
		}

		// ── Step 2: find the exact qcow2 filename in the Cloud images dir ─────
		imagesURL := fmt.Sprintf(
			"https://dl.fedoraproject.org/pub/fedora/linux/releases/%d/Cloud/%s/images/",
			version, fArch,
		)
		body, err = fetchText(imagesURL)
		if err != nil {
			return "", "", fmt.Errorf("fedora: listing images for v%d/%s: %w", version, fArch, err)
		}

		filename, err := parseFedoraImageFilename(body, fArch, version)
		if err != nil {
			return "", "", fmt.Errorf("fedora: %w", err)
		}

		downloadURL := imagesURL + filename

		// ── Step 3: fetch checksum ─────────────────────────────────────────────
		// Fedora publishes per-file checksums as <filename>-CHECKSUM files.
		checksumURL := imagesURL + filename + "-CHECKSUM"
		checksum, err := parseFedoraChecksum(checksumURL, filename)
		if err != nil {
			// Non-fatal: proceed without verification.
			return downloadURL, "", nil
		}

		return downloadURL, checksum, nil
	}
}

// versionDirRE matches directory links like href="42/" in HTML listings.
var versionDirRE = regexp.MustCompile(`href="(\d+)/"`)

// parseFedoraLatestVersion extracts the highest stable Fedora version number
// from the HTML directory listing of the releases page.
func parseFedoraLatestVersion(html string) (int, error) {
	matches := versionDirRE.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("could not find any version directories in releases listing")
	}

	max := 0
	for _, m := range matches {
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		// Skip pre-release versions (historically numbered > 100 or similar
		// signals). Anything ≥ 100 is treated as non-stable.
		if v < 100 && v > max {
			max = v
		}
	}

	if max == 0 {
		return 0, fmt.Errorf("could not determine latest stable version from releases listing")
	}

	return max, nil
}

// fedoraImageRE matches filenames like Fedora-Cloud-Base-Generic.x86_64-42-1.1.qcow2
var fedoraImageRE = regexp.MustCompile(
	`(Fedora-Cloud-Base-Generic\.[^"]+\.qcow2)`)

// parseFedoraImageFilename finds the Cloud-Base-Generic qcow2 filename
// from an HTML directory listing page.
func parseFedoraImageFilename(html, fArch string, version int) (string, error) {
	matches := fedoraImageRE.FindAllStringSubmatch(html, -1)
	needle := fmt.Sprintf(".%s-%d-", fArch, version)

	for _, m := range matches {
		name := m[1]
		if strings.Contains(name, needle) {
			return name, nil
		}
	}

	return "", fmt.Errorf(
		"could not find Fedora-Cloud-Base-Generic qcow2 for %s v%d in images listing",
		fArch, version)
}

// sha256LineRE matches lines in Fedora CHECKSUM files:
// SHA256 (<filename>) = <hash>
var sha256LineRE = regexp.MustCompile(`SHA256 \(([^)]+)\) = ([0-9a-f]+)`)

// parseFedoraChecksum fetches a Fedora per-file CHECKSUM file and extracts
// the SHA256 hash for the given filename.
func parseFedoraChecksum(checksumURL, filename string) (string, error) {
	body, err := fetchText(checksumURL)
	if err != nil {
		return "", err
	}

	for _, m := range sha256LineRE.FindAllStringSubmatch(body, -1) {
		if m[1] == filename {
			return m[2], nil
		}
	}

	return "", fmt.Errorf("checksum not found for %q in %s", filename, checksumURL)
}
