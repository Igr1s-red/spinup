package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CopyID installs a local SSH public key into the VM's authorized_keys.
// If keyPath is empty, it searches ~/.ssh/ for the first id_*.pub file.
// After this, standard SSH clients can connect without spinup's managed key.
func (v *VirtualMachine) CopyID(keyPath string) error {
	if err := v.requireRunning(); err != nil {
		return err
	}

	pubKey, err := loadPublicKey(keyPath)
	if err != nil {
		return err
	}

	// Ensure ~/.ssh exists with correct permissions, then append the key
	// idempotently (skip if already present).
	script := fmt.Sprintf(`
set -e
mkdir -p ~/.ssh
chmod 700 ~/.ssh
touch ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
key=%q
if ! grep -qF "$key" ~/.ssh/authorized_keys; then
    echo "$key" >> ~/.ssh/authorized_keys
    echo "Key added."
else
    echo "Key already present — nothing to do."
fi
`, strings.TrimSpace(string(pubKey)))

	client, err := v.sshClient()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = os.Stderr

	if err := session.Run(script); err != nil {
		return fmt.Errorf("install key: %w", err)
	}

	v.engine.printf("%s", out.String())
	return nil
}

// loadPublicKey reads a public key from keyPath, or discovers the first
// id_*.pub file in ~/.ssh/ when keyPath is empty.
func loadPublicKey(keyPath string) ([]byte, error) {
	if keyPath != "" {
		b, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read public key %s: %w", keyPath, err)
		}
		return b, nil
	}

	// Auto-discover: try common key file names in order of preference.
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	candidates := []string{
		"id_ed25519.pub",
		"id_rsa.pub",
		"id_ecdsa.pub",
		"id_dsa.pub",
	}

	for _, name := range candidates {
		path := filepath.Join(home, ".ssh", name)
		if b, err := os.ReadFile(path); err == nil {
			return b, nil
		}
	}

	return nil, ErrCopyIDNotFound
}
