package engine

import "fmt"

// Pause suspends the VM's CPU execution via QMP (QMP 'stop').
// The QEMU process stays alive; use Resume to continue execution.
func (v *VirtualMachine) Pause() error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	qmp, err := v.connectQMP()
	if err != nil {
		return err
	}
	defer qmp.Close()
	if err := qmp.Pause(); err != nil {
		return fmt.Errorf("pause: %w", err)
	}
	v.engine.printf("VM %q paused\n", v.Name)
	return nil
}

// Resume resumes a paused VM's CPU via QMP (QMP 'cont').
func (v *VirtualMachine) Resume() error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	qmp, err := v.connectQMP()
	if err != nil {
		return err
	}
	defer qmp.Close()
	if err := qmp.Resume(); err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	v.engine.printf("VM %q resumed\n", v.Name)
	return nil
}
