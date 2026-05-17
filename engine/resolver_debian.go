package engine

// Debian cloud image resolver.
//
// Debian publishes official cloud images at:
//   https://cloud.debian.org/images/cloud/<codename>/latest/
//
// The directory always contains:
//   - debian-<N>-generic-<arch>-<date>-<build>.qcow2
//   - SHA512SUMS (one line per file: "<hash>  [*]<filename>")
//
// We fetch SHA512SUMS, locate the line whose filename matches
// "generic-<arch>" + ".qcow2", then construct the download URL from the
// same directory. This means we always get the latest nightly build without
// any hardcoded date stamps.

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// debianGoarchMap translates Go arch names to the Debian cloud image arch
// component used in filenames.
var debianGoarchMap = map[string]string{
	"amd64": "amd64",
	"arm64": "arm64",
}

// resolveDebian returns a Resolver function for the named Debian codename.
func resolveDebian(codename string) func(goarch string) (string, string, error) {
	return func(goarch string) (string, string, error) {
		debArch, ok := debianGoarchMap[goarch]
		if !ok {
			return "", "", fmt.Errorf("debian: unsupported architecture %q", goarch)
		}

		baseURL := fmt.Sprintf(
			"https://cloud.debian.org/images/cloud/%s/latest", codename)
		sumsURL := baseURL + "/SHA512SUMS"

		resp, err := http.Get(sumsURL)
		if err != nil {
			return "", "", fmt.Errorf("debian: fetch SHA512SUMS: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf(
				"debian: SHA512SUMS returned HTTP %d for %s", resp.StatusCode, sumsURL)
		}

		filename, checksum, err := parseDebianSHA512SUMS(resp.Body, debArch)
		if err != nil {
			return "", "", fmt.Errorf("debian: %w for %s", err, codename)
		}

		return fmt.Sprintf("%s/%s", baseURL, filename), checksum, nil
	}
}

// parseDebianSHA512SUMS finds the filename and SHA-512 checksum for the given
// Debian arch from a SHA512SUMS file. Lines have the form "<hash>  [*]<filename>".
func parseDebianSHA512SUMS(r io.Reader, debArch string) (filename, checksum string, err error) {
	scanner := bufio.NewScanner(r)
	needle := fmt.Sprintf("generic-%s", debArch)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		fname := strings.TrimLeft(parts[1], "*")
		if strings.Contains(fname, needle) && strings.HasSuffix(fname, ".qcow2") {
			return fname, parts[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("scanning SHA512SUMS: %w", err)
	}
	return "", "", fmt.Errorf("could not find generic-%s qcow2 in SHA512SUMS", debArch)
}

// fetchText is a small helper used by multiple resolvers.
func fetchText(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024)) // 2 MiB cap
	if err != nil {
		return "", err
	}
	return string(b), nil
}
