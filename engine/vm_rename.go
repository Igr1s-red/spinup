package engine

import (
	"fmt"
	"os"
)

// RenameVirtualMachine moves a VM's data directory from oldName to newName.
// The VM must be stopped. Any existing SSH config block for the old name is
// removed before the move (non-fatal if absent).
func (e *Engine) RenameVirtualMachine(oldName, newName string) (*VirtualMachine, error) {
	vm := e.FindVirtualMachine(oldName)
	if vm == nil {
		return nil, fmt.Errorf("engine: VM %q not found", oldName)
	}
	if err := vm.requireStopped(); err != nil {
		return nil, err
	}
	if !validName.MatchString(newName) {
		return nil, ErrInvalidName
	}
	if e.FindVirtualMachine(newName) != nil {
		return nil, ErrVirtualMachineAlreadyExist
	}

	if err := vm.SSHConfigRemove(); err != nil {
		e.printf("Warning: could not remove SSH config for %q: %s\n", oldName, err)
	}

	newPath := e.virtualMachinePath(newName)
	if err := os.Rename(vm.path, newPath); err != nil {
		return nil, fmt.Errorf("rename VM directory: %w", err)
	}

	delete(e.virtualMachines, oldName)
	vm.Name = newName
	vm.path = newPath
	e.virtualMachines[newName] = vm

	e.printf("Renamed %q → %q\n", oldName, newName)
	return vm, nil
}
