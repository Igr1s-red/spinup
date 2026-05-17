package engine

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
)

// CloneVirtualMachine creates a new VM as a copy-on-write linked clone of src.
// The source VM must be stopped. Only blocks that differ from the source are
// written to the clone's qcow2 disk.
func (e *Engine) CloneVirtualMachine(srcName, dstName string) (*VirtualMachine, error) {
	src := e.FindVirtualMachine(srcName)
	if src == nil {
		return nil, fmt.Errorf("engine: source VM %q not found", srcName)
	}
	if err := src.requireStopped(); err != nil {
		if err == ErrVirtualMachineAlreadyRunning {
			return nil, ErrCloneSourceRunning
		}
		return nil, err
	}
	if !validName.MatchString(dstName) {
		return nil, ErrInvalidName
	}
	if e.FindVirtualMachine(dstName) != nil {
		return nil, ErrVirtualMachineAlreadyExist
	}

	e.printf("Cloning %q → %q\n", srcName, dstName)

	// ── Build NIC configs ─────────────────────────────────────────────────────
	dstNetworks := make([]NetworkInterfaceConfig, len(src.Config.Networks))
	for i, n := range src.Config.Networks {
		mac, err := RandomMAC()
		if err != nil {
			return nil, err
		}
		pf := make(map[string]string, len(n.PortForwards))
		for vmPort := range n.PortForwards {
			if vmPort == "22" {
				continue // assigned separately below
			}
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return nil, fmt.Errorf("allocate port for clone: %w", err)
			}
			pf[vmPort] = strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
			l.Close()
		}
		dstNetworks[i] = NetworkInterfaceConfig{MACAddress: mac, NetworkMode: n.NetworkMode, PortForwards: pf}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("allocate SSH port for clone: %w", err)
	}
	sshPort := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	l.Close()
	dstNetworks[0].PortForwards["22"] = sshPort
	e.printf("Clone SSH port: %s\n", sshPort)

	dst := &VirtualMachine{
		Name:   dstName,
		engine: e,
		path:   e.virtualMachinePath(dstName),
		Config: VirtualMachineConfig{
			CPU:           src.Config.CPU,
			Memory:        src.Config.Memory,
			Image:         src.Config.Image,
			SSHUser:       src.Config.SSHUser,
			DiskSize:      src.Config.DiskSize,
			Networks:      dstNetworks,
			SharedFolders: src.Config.SharedFolders,
		},
	}

	// ── Ed25519 SSH key ───────────────────────────────────────────────────────
	e.printf("Generating Ed25519 SSH key for clone\n")
	pubKeyBytes, err := dst.generateSSHKey()
	if err != nil {
		return nil, err
	}

	// ── Linked clone disk ─────────────────────────────────────────────────────
	e.printf("Creating linked clone disk (copy-on-write)\n")
	out, err := exec.Command("qemu-img", "create",
		"-f", "qcow2", "-F", "qcow2",
		"-b", src.diskPath(),
		dst.diskPath(),
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("qemu-img create: %w\n%s", err, out)
	}

	// ── Cloud-init ISO ────────────────────────────────────────────────────────
	e.printf("Building cloud-init ISO for clone\n")
	if err := dst.buildCloudInit(defaultCloudInitUserData(pubKeyBytes), dst.Name); err != nil {
		return nil, fmt.Errorf("build cloud-init for clone: %w", err)
	}

	if err := dst.writeConfigFile(); err != nil {
		return nil, err
	}

	e.virtualMachines[dstName] = dst
	e.printf("Clone %q ready\n", dstName)
	return dst, nil
}
