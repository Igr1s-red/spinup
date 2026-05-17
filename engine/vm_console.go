package engine

import (
	"fmt"
	"os"
	"os/exec"
)

// Console connects stdin/stdout to the VM's serial console via socat.
// The VM must be running and have been started with a console socket
// (all VMs started with current versions of spinup have this).
// Press Ctrl+] to disconnect from the console.
func (v *VirtualMachine) Console() error {
	if err := v.requireRunning(); err != nil {
		return err
	}

	socatPath, err := exec.LookPath("socat")
	if err != nil {
		return ErrSocatNotFound
	}

	sockPath := v.consoleSocketPath()
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		return ErrConsoleNotAvailable
	}

	v.engine.printf("Connecting to serial console of %q (Ctrl+] to exit)\n", v.Name)

	// escape=0x1d causes socat to exit on Ctrl+] (ASCII GS, 0x1d).
	cmd := exec.Command(socatPath,
		"-,raw,echo=0,escape=0x1d",
		"UNIX-CONNECT:"+sockPath,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// socat exits 1 when the user presses the escape key — not an error.
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("socat: %w", err)
	}
	return nil
}
