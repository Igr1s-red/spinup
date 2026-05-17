package engine

import (
	"bytes"
	"fmt"
	"strings"
)

// IP returns the primary IP address of the VM by running `hostname -I` via SSH.
// The VM must be running.
func (v *VirtualMachine) IP() (string, error) {
	if err := v.requireRunning(); err != nil {
		return "", err
	}

	client, err := v.sshClient()
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	if err := session.Run("hostname -I"); err != nil {
		// hostname -I is Linux-specific; try ip route on older systems
		session2, _ := client.NewSession()
		if session2 != nil {
			var out2 bytes.Buffer
			session2.Stdout = &out2
			session2.Run("ip route get 1.1.1.1 | awk '{print $7}' | head -1")
			session2.Close()
			if addr := strings.TrimSpace(out2.String()); addr != "" {
				return addr, nil
			}
		}
		return "", fmt.Errorf("get IP: %w", err)
	}

	// hostname -I returns a space-separated list; first entry is primary.
	addrs := strings.Fields(out.String())
	if len(addrs) == 0 {
		return "", fmt.Errorf("VM reported no IP addresses")
	}
	return addrs[0], nil
}

// AllIPs returns all IP addresses reported by the VM.
func (v *VirtualMachine) AllIPs() ([]string, error) {
	if err := v.requireRunning(); err != nil {
		return nil, err
	}

	client, err := v.sshClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	if err := session.Run("hostname -I"); err != nil {
		return nil, fmt.Errorf("get IPs: %w", err)
	}

	addrs := strings.Fields(out.String())
	if len(addrs) == 0 {
		return nil, fmt.Errorf("VM reported no IP addresses")
	}
	return addrs, nil
}
