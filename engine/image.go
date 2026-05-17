package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Image represents a named OS image available to create VMs from.
// All images use a Resolver to fetch the current URL and checksum at pull time.
type Image struct {
	Description string
	Name        string
	// Version is a human-readable label (codename or "latest").
	Version string
	// Dynamic is always true for resolver-backed images; shown in `spinup images`.
	Dynamic bool
	// Resolver is called at Pull() time to obtain the download URL and an
	// optional checksum for the host architecture (runtime.GOARCH).
	// A returned empty checksum skips verification.
	Resolver func(goarch string) (url, checksum string, err error)

	engine  *Engine
	path    string
	sshUser string
}

// diskPath is the local path of the downloaded image file.
func (i *Image) diskPath() string { return filepath.Join(i.path, "disk.qcow2") }

// Pulled reports whether the image has already been downloaded.
func (i *Image) Pulled() (bool, error) {
	_, err := os.Stat(i.diskPath())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat image disk: %w", err)
	}
	return true, nil
}

// Pull downloads and verifies the image for the current host architecture.
func (i *Image) Pull() error {
	if i.Resolver == nil {
		return fmt.Errorf("image %s:%s has no resolver configured", i.Name, i.Version)
	}

	i.engine.printf("Pulling %s:%s\n", i.Name, i.Version)

	if err := os.MkdirAll(i.path, 0755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}

	i.engine.printf("Resolving latest %s:%s for %s...\n", i.Name, i.Version, runtime.GOARCH)
	url, checksum, err := i.Resolver(runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("resolve %s:%s: %w", i.Name, i.Version, err)
	}
	i.engine.printf("Resolved: %s\n", url)

	if err := i.engine.download(url, i.diskPath()); err != nil {
		return err
	}

	return i.engine.verifyChecksum(checksum, i.diskPath())
}

// Update re-downloads the image, replacing the cached disk.
// Existing VMs created from the old image are not affected —
// they use a copy of the disk, not a reference to this file.
func (i *Image) Update() error {
	if err := os.Remove(i.diskPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old disk: %w", err)
	}
	i.engine.printf("Re-pulling %s:%s\n", i.Name, i.Version)
	return i.Pull()
}

// Remove deletes the downloaded image disk to reclaim disk space.
// Existing VMs already created from the image are not affected — they use a copy.
func (i *Image) Remove() error {
	pulled, err := i.Pulled()
	if err != nil {
		return err
	}
	if !pulled {
		return fmt.Errorf("image %s:%s is not pulled", i.Name, i.Version)
	}
	if err := os.RemoveAll(i.path); err != nil {
		return fmt.Errorf("remove image %s:%s: %w", i.Name, i.Version, err)
	}
	i.engine.printf("Image %s:%s removed\n", i.Name, i.Version)
	return nil
}
