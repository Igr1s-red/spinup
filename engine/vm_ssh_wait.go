package engine

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// WaitForSSH blocks until the VM's SSH port accepts connections and the stored
// key is accepted, or until ctx is cancelled.
//
// Use context.WithTimeout to set a deadline:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
//	defer cancel()
//	vm.WaitForSSH(ctx)
//
// The wait has two phases:
//  1. TCP dial (fast): confirms sshd is listening.
//  2. Auth handshake (slower): confirms cloud-init has applied the key.
func (v *VirtualMachine) WaitForSSH(ctx context.Context) error {
	port := v.Config.sshPort()
	if port == "" {
		return ErrInvalidSSHPort
	}

	addr := fmt.Sprintf("127.0.0.1:%s", port)
	v.engine.printf("Waiting for SSH on %s", addr)

	// Phase 1: wait for TCP port to open.
	for {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			break
		}
		v.engine.printf(".")
		select {
		case <-ctx.Done():
			v.engine.printf(" timed out waiting for sshd\n")
			return ErrSSHTimeout
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Phase 2: attempt a real SSH auth handshake.
	// Cloud-init installs authorized_keys after sshd is already running,
	// so we retry until auth succeeds or ctx is done.
	signer, err := v.loadSigner()
	if err != nil {
		// Can't verify auth — TCP is open, best-effort return.
		v.engine.printf(" ready (TCP)\n")
		return nil
	}

	cfg := &ssh.ClientConfig{
		User:            v.Config.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         3 * time.Second,
	}

	for {
		client, err := ssh.Dial("tcp", addr, cfg)
		if err == nil {
			client.Close()
			v.engine.printf(" ready\n")
			return nil
		}

		v.engine.printf(".")
		select {
		case <-ctx.Done():
			v.engine.printf(" timed out waiting for key auth\n")
			return ErrSSHAuthTimeout
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// WaitForStatus polls Status() until the VM reaches target or ctx is done.
func (v *VirtualMachine) WaitForStatus(ctx context.Context, target VirtualMachineStatus) error {
	for {
		s, err := v.Status()
		if err != nil {
			return err
		}
		if s == target {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for VM %q to be %s", v.Name, target)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// WaitForPort blocks until the given host-side TCP port accepts connections or ctx is done.
func (v *VirtualMachine) WaitForPort(ctx context.Context, port string) error {
	addr := "127.0.0.1:" + port
	v.engine.printf("Waiting for port %s on %q", port, v.Name)
	for {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			v.engine.printf(" ready\n")
			return nil
		}
		v.engine.printf(".")
		select {
		case <-ctx.Done():
			v.engine.printf(" timed out\n")
			return fmt.Errorf("timed out waiting for port %s on %q", port, v.Name)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// loadSigner reads and parses the VM's private key for use in SSH auth.
func (v *VirtualMachine) loadSigner() (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(v.privateKeyPath())
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}
