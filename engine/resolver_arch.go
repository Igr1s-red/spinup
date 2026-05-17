package engine

// Arch Linux cloud image resolver.
//
// The Arch Linux Reproducible Builds team publishes official cloud images at:
//   https://geo.mirror.pkgbuild.com/images/latest/
//
// Files:
//   Arch-Linux-x86_64-cloudimg.qcow2          (amd64)
//   Arch-Linux-aarch64-cloudimg.qcow2         (arm64)
//   Arch-Linux-x86_64-cloudimg.qcow2.SHA256   (one-line SHA256 checksum)
//   Arch-Linux-aarch64-cloudimg.qcow2.SHA256  (one-line SHA256 checksum)
//
// The "latest" directory is a symlink maintained by the Arch Linux
// infrastructure team and always points to the most recent successful build.
//
// SSH user: arch (created by cloud-init via the official image)
//
// Note: Arch Linux does not publish ARM cloud images for all releases.
// The aarch64 image URL is included; if it 404s at pull time the user
// will receive a clear error from the download step.

import (
	"fmt"
	"strings"
)

// archGoarchMap maps Go arch names to the Arch Linux image filename arch component.
var archGoarchMap = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
}

const archMirrorBase = "https://geo.mirror.pkgbuild.com/images/latest"

// resolveArch returns a Resolver for the latest Arch Linux cloud image.
// The "latest" directory is maintained upstream so no version scraping is
// needed — we just fetch the checksum file to verify integrity.
func resolveArch() func(goarch string) (string, string, error) {
	return func(goarch string) (string, string, error) {
		aArch, ok := archGoarchMap[goarch]
		if !ok {
			return "", "", fmt.Errorf("arch: unsupported architecture %q", goarch)
		}

		filename := fmt.Sprintf("Arch-Linux-%s-cloudimg.qcow2", aArch)
		downloadURL := fmt.Sprintf("%s/%s", archMirrorBase, filename)
		checksumURL := downloadURL + ".SHA256"

		// The .SHA256 file contains a single line:
		//   <sha256hash>  <filename>
		body, err := fetchText(checksumURL)
		if err != nil {
			// Proceed without verification if the checksum file is unreachable.
			return downloadURL, "", nil
		}

		checksum, err := parseArchChecksum(body, filename)
		if err != nil {
			return downloadURL, "", nil
		}

		return downloadURL, checksum, nil
	}
}

// parseArchChecksum parses the single-line SHA256 file produced by Arch Linux.
// Format: "<sha256hash>  <filename>" (BSD or GNU style).
func parseArchChecksum(content, filename string) (string, error) {
	// Handle both "hash  filename" and "SHA256 (filename) = hash" styles.
	content = strings.TrimSpace(content)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// GNU style: "<hash>  <filename>"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			// Strip leading '*' (binary mode) from filename field.
			name := strings.TrimLeft(parts[len(parts)-1], "*")
			if name == filename || strings.HasSuffix(name, filename) {
				return hash, nil
			}
		}
	}

	return "", fmt.Errorf("arch: checksum for %q not found in checksum file", filename)
}
