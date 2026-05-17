package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Mount mounts a remote VM directory on the host using sshfs.
// Requires sshfs to be installed:
//
//	macOS: brew install macfuse sshfs
//	Linux: sudo apt install sshfs
func (v *VirtualMachine) Mount(remotePath, localDir string) error {
	if err := v.requireRunning(); err != nil {
		return err
	}

	if _, err := exec.LookPath("sshfs"); err != nil {
		return ErrSshfsNotFound
	}

	port := v.Config.sshPort()
	if port == "" {
		return ErrInvalidSSHPort
	}

	absLocal, err := filepath.Abs(localDir)
	if err != nil {
		return fmt.Errorf("resolve local path: %w", err)
	}

	if err := os.MkdirAll(absLocal, 0755); err != nil {
		return fmt.Errorf("create mount point %s: %w", absLocal, err)
	}

	// Build the sshfs remote string: user@host:path
	remote := fmt.Sprintf("%s@127.0.0.1:%s", v.Config.SSHUser, remotePath)

	args := []string{
		remote, absLocal,
		"-p", port,
		"-o", fmt.Sprintf("IdentityFile=%s", v.privateKeyPath()),
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		// macOS: allow_other is not needed; reconnect helps with sleep/wake
		"-o", "reconnect",
	}

	// macOS: volname makes it appear nicely in Finder
	if isMacOS() {
		args = append(args, "-o", fmt.Sprintf("volname=%s-%s", v.Name, filepath.Base(remotePath)))
	}

	cmd := exec.Command("sshfs", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sshfs %s -> %s: %w", remote, absLocal, err)
	}

	v.engine.printf("Mounted %s:%s → %s\n", v.Name, remotePath, absLocal)
	return nil
}

// Unmount unmounts an sshfs mount point.
func (v *VirtualMachine) Unmount(localDir string) error {
	if err := unmountDir(localDir); err != nil {
		return err
	}
	abs, _ := filepath.Abs(localDir)
	v.engine.printf("Unmounted %s\n", abs)
	return nil
}

// isMacOS reports whether the current OS is macOS.
func isMacOS() bool { return runtime.GOOS == "darwin" }

// UnmountDir unmounts a local sshfs mount point.
// Package-level so cmd can call it without a VirtualMachine reference.
func UnmountDir(localDir string) error {
	return unmountDir(localDir)
}

func unmountDir(localDir string) error {
	abs, err := filepath.Abs(localDir)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	var cmd *exec.Cmd
	if isMacOS() {
		// diskutil handles FUSE mounts cleanly on macOS
		cmd = exec.Command("diskutil", "unmount", "force", abs)
	} else {
		if _, err := exec.LookPath("fusermount"); err == nil {
			cmd = exec.Command("fusermount", "-u", abs)
		} else {
			cmd = exec.Command("umount", abs)
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unmount %s: %w", abs, err)
	}
	return nil
}
