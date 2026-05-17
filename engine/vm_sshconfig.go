package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sshConfigPath = "~/.ssh/config"

// sshConfigMarker is written as a comment so spinup can locate its own blocks.
const sshConfigBlockStart = "# spinup-managed: %s"
const sshConfigBlockEnd = "# spinup-managed-end: %s"

// SSHConfigEntry writes or updates an SSH config block for the VM in
// ~/.ssh/config. Existing entries for the same VM name are replaced atomically.
// The VM must be running so the SSH port is known.
func (v *VirtualMachine) SSHConfigEntry() error {
	if err := v.requireRunning(); err != nil {
		return err
	}

	port := v.Config.sshPort()
	if port == "" {
		return ErrInvalidSSHPort
	}

	block := fmt.Sprintf(
		"%s\nHost %s\n    HostName 127.0.0.1\n    Port %s\n    User %s\n"+
			"    IdentityFile %s\n    IdentitiesOnly yes\n"+
			"    StrictHostKeyChecking no\n    UserKnownHostsFile /dev/null\n%s\n",
		fmt.Sprintf(sshConfigBlockStart, v.Name),
		v.Name,
		port,
		v.Config.SSHUser,
		v.privateKeyPath(),
		fmt.Sprintf(sshConfigBlockEnd, v.Name),
	)

	cfgPath := expandHome(sshConfigPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
		return fmt.Errorf("create ~/.ssh directory: %w", err)
	}

	existing, err := readFileOrEmpty(cfgPath)
	if err != nil {
		return err
	}

	updated := removeSSHBlock(existing, v.Name) + "\n" + block

	if err := writeFileAtomic(cfgPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("write %s: %w", cfgPath, err)
	}

	v.engine.printf("SSH config written for %q → ssh %s\n", v.Name, v.Name)
	return nil
}

// SSHConfigRemove removes the spinup-managed SSH config block for this VM.
func (v *VirtualMachine) SSHConfigRemove() error {
	cfgPath := expandHome(sshConfigPath)

	existing, err := readFileOrEmpty(cfgPath)
	if err != nil {
		return err
	}

	// Nothing to do if the block isn't present (avoids creating the file).
	if !strings.Contains(existing, fmt.Sprintf(sshConfigBlockStart, v.Name)) {
		return nil
	}

	updated := removeSSHBlock(existing, v.Name)
	if err := writeFileAtomic(cfgPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("write %s: %w", cfgPath, err)
	}

	v.engine.printf("SSH config entry removed for %q\n", v.Name)
	return nil
}

// removeSSHBlock strips the spinup-managed block for vmName from content.
func removeSSHBlock(content, vmName string) string {
	start := fmt.Sprintf(sshConfigBlockStart, vmName)
	end := fmt.Sprintf(sshConfigBlockEnd, vmName)

	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	inBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == start {
			inBlock = true
			continue
		}
		if strings.TrimSpace(line) == end {
			inBlock = false
			continue
		}
		if !inBlock {
			out.WriteString(line + "\n")
		}
	}

	// Remove any trailing blank lines added by previous writes.
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// ── helpers ───────────────────────────────────────────────────────────────────

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func readFileOrEmpty(path string) (string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(b), nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".spinup.tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
