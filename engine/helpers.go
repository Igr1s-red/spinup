package engine

// helpers.go — small package-level utilities shared across engine files.

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// copyFileTo copies src to dst, creating dst if needed.
// On write failure the partial destination file is removed.
func copyFileTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	return out.Close()
}

// runQemuImg runs qemu-img with the given arguments and returns combined output.
func runQemuImg(args ...string) ([]byte, error) {
	return exec.Command("qemu-img", args...).CombinedOutput()
}

// randomHex returns n random bytes encoded as a lowercase hex string.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// scpCmd builds an scp exec.Cmd with standard spinup options.
func scpCmd(keyPath, port, src, dst string) *exec.Cmd {
	return exec.Command("scp",
		"-i", keyPath,
		"-P", port,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "IdentitiesOnly=yes",
		src, dst,
	)
}
