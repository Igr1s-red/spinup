package engine

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// Export converts the VM's disk to a standalone compressed qcow2 file.
// The VM must be stopped. Any backing-file chain (linked clone) is collapsed
// so the output is self-contained.
// If outPath is empty it defaults to "<name>.qcow2" in the working directory.
func (v *VirtualMachine) Export(outPath string) error {
	if err := v.requireStopped(); err != nil {
		return err
	}
	if outPath == "" {
		outPath = v.Name + ".qcow2"
	}
	v.engine.printf("Exporting VM %q → %s\n", v.Name, outPath)
	// -c compresses blocks; backing-file chain collapsed by full conversion.
	out, err := runQemuImg("convert", "-c", "-f", "qcow2", "-O", "qcow2",
		v.diskPath(), outPath)
	if err != nil {
		return fmt.Errorf("qemu-img convert: %w\n%s", err, out)
	}
	v.engine.printf("Exported to %s\n", outPath)
	return nil
}

// ImportOptions configures how to import an external qcow2 disk as a new VM.
type ImportOptions struct {
	// DiskPath is the path to the qcow2 file to import.
	DiskPath string
	// Name is the name for the new VM.
	Name string
	// Image selects the SSH user (e.g. "ubuntu:jammy"). Defaults to "debian:bookworm".
	Image string
	// CPU and Memory set the VM's resources; zero values use sensible defaults.
	CPU    int
	Memory int
}

// ImportVirtualMachine creates a new VM from an externally-provided qcow2 disk.
// A fresh Ed25519 SSH key and cloud-init ISO are generated; the guest must be
// cloud-init enabled and accept the new key on first boot.
func (e *Engine) ImportVirtualMachine(opts ImportOptions) (*VirtualMachine, error) {
	if opts.Name == "" || !validName.MatchString(opts.Name) {
		return nil, ErrInvalidName
	}
	if e.FindVirtualMachine(opts.Name) != nil {
		return nil, ErrVirtualMachineAlreadyExist
	}
	if opts.CPU < 1 {
		opts.CPU = 2
	}
	if opts.Memory < 128 {
		opts.Memory = 1024
	}
	imageKey := opts.Image
	if imageKey == "" {
		imageKey = "debian:bookworm"
	}
	img := e.FindImage(imageKey)
	if img == nil {
		return nil, fmt.Errorf("%w: %q", ErrImageNotFound, imageKey)
	}
	if _, err := os.Stat(opts.DiskPath); err != nil {
		return nil, fmt.Errorf("disk file: %w", err)
	}

	e.printf("Importing VM %q from %s\n", opts.Name, opts.DiskPath)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("allocate SSH port: %w", err)
	}
	sshPort := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	l.Close()

	mac, err := RandomMAC()
	if err != nil {
		return nil, err
	}

	vm := &VirtualMachine{
		Name:   opts.Name,
		engine: e,
		path:   e.virtualMachinePath(opts.Name),
		Config: VirtualMachineConfig{
			CPU:     opts.CPU,
			Memory:  opts.Memory,
			Image:   imageKey,
			SSHUser: img.sshUser,
			Networks: []NetworkInterfaceConfig{{
				MACAddress:   mac,
				NetworkMode:  "nat",
				PortForwards: map[string]string{"22": sshPort},
			}},
		},
	}

	pubKeyBytes, err := vm.generateSSHKey()
	if err != nil {
		return nil, err
	}

	e.printf("Copying disk\n")
	if err := copyFileTo(opts.DiskPath, vm.diskPath()); err != nil {
		return nil, err
	}

	e.printf("Building cloud-init ISO\n")
	if err := vm.buildCloudInit(defaultCloudInitUserData(pubKeyBytes), vm.Name); err != nil {
		return nil, fmt.Errorf("build cloud-init: %w", err)
	}

	if err := vm.writeConfigFile(); err != nil {
		return nil, err
	}

	e.virtualMachines[opts.Name] = vm
	e.printf("VM %q imported\n", opts.Name)
	return vm, nil
}
