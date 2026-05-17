package qemu

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const (
	Aarch64ExecutableName = "qemu-system-aarch64"
	X8664ExecutableName   = "qemu-system-x86_64"
)

// NetworkMode controls how a NIC connects to the network.
type NetworkMode string

const (
	// NetworkModeNAT uses QEMU user-mode networking with NAT.
	// No privileges required. Port forwards are supported.
	NetworkModeNAT NetworkMode = "nat"

	// NetworkModeBridged attaches the VM directly to a physical interface.
	// Requires root / CAP_NET_ADMIN on Linux; vmnet entitlement on macOS.
	NetworkModeBridged NetworkMode = "bridged"

	// NetworkModeInternal isolates the VM from external traffic.
	// The VM still gets a DHCP lease but all outbound routing is blocked.
	NetworkModeInternal NetworkMode = "internal"

	// NetworkModeHostOnly allows VM ↔ host communication only.
	// Uses vmnet-host on macOS; a restricted user subnet on Linux.
	NetworkModeHostOnly NetworkMode = "host-only"
)

// NetworkInterface describes one NIC to attach to the VM.
type NetworkInterface struct {
	MACAddress   string
	Mode         NetworkMode
	PortForwards map[string]string // only used by nat and internal modes
}

// SharedFolder describes a host directory to expose inside the VM via
// VirtIO 9P. Mount inside the guest with:
//
//	mount -t 9p -o trans=virtio,version=9p2000.L <Tag> /mnt/point
type SharedFolder struct {
	HostPath string
	Tag      string
}

type CommandOptionsDisk struct {
	Path     string
	ReadOnly bool
}

type CommandOptions struct {
	CPU    int
	Memory int
	Disks  []CommandOptionsDisk
	// NICs to attach. At least one is required.
	Interfaces []NetworkInterface
	// SharedFolders are host directories exposed via VirtIO 9P.
	SharedFolders []SharedFolder
	// MonitorSocketPath, if non-empty, exposes a QMP monitor on that path.
	MonitorSocketPath string
	// ConsoleSocketPath, if non-empty, routes the VM serial console to a Unix
	// socket (accessible via spinup console). When empty the serial falls back
	// to the process's stdio (recorded in qemu.log).
	ConsoleSocketPath string
}

type NewOptions struct {
	ExecutableName string
}

type QEMU struct {
	accelerator    string
	bios           string
	cpu            string
	executableName string
	machine        string
}

func (q *QEMU) Command(opts CommandOptions) (*exec.Cmd, error) {
	cmdArgs := []string{
		"-machine", q.machine,
		"-m", fmt.Sprint(opts.Memory),
		"-cpu", q.cpu,
		"-smp", fmt.Sprint(opts.CPU),
		"-accel", q.accelerator,
		"-device", "virtio-rng-pci",
		"-device", "virtio-balloon-pci", // enables memory balloon / stats
		"-display", "none",
		"-nodefaults",
	}

	// Serial console: route to a socket when a path is provided so
	// `spinup console` can attach interactively; otherwise fall back to
	// stdio (captured in qemu.log).
	if opts.ConsoleSocketPath != "" {
		cmdArgs = append(cmdArgs,
			"-chardev", fmt.Sprintf("socket,id=console,path=%s,server=on,wait=off", opts.ConsoleSocketPath),
			"-serial", "chardev:console",
		)
	} else {
		cmdArgs = append(cmdArgs, "-serial", "mon:stdio")
	}

	if q.bios != "" {
		cmdArgs = append(cmdArgs, "-bios", q.bios)
	}

	// ── QMP monitor socket ────────────────────────────────────────────────────
	if opts.MonitorSocketPath != "" {
		cmdArgs = append(cmdArgs,
			"-monitor", fmt.Sprintf("unix:%s,server,nowait", opts.MonitorSocketPath),
		)
	}

	// ── Disks ─────────────────────────────────────────────────────────────────
	for i, disk := range opts.Disks {
		blockDevOpts := []string{fmt.Sprintf("node-name=drive%d", i)}

		if strings.HasSuffix(disk.Path, ".qcow2") {
			blockDevOpts = append(blockDevOpts, "driver=qcow2", "file.driver=file",
				fmt.Sprintf("file.filename=%s", disk.Path))
		} else {
			blockDevOpts = append(blockDevOpts, "driver=raw", "file.driver=file",
				fmt.Sprintf("file.filename=%s", disk.Path))
		}

		if disk.ReadOnly {
			blockDevOpts = append(blockDevOpts, "read-only=on")
		}

		cmdArgs = append(cmdArgs,
			"-device", fmt.Sprintf("virtio-blk,drive=drive%d", i),
			"-blockdev", strings.Join(blockDevOpts, ","),
		)
	}

	// ── Network interfaces ────────────────────────────────────────────────────
	for i, iface := range opts.Interfaces {
		netdevID := fmt.Sprintf("netdev%d", i)
		netArgs, err := q.buildIfaceArgs(netdevID, iface)
		if err != nil {
			return nil, err
		}
		cmdArgs = append(cmdArgs, netArgs...)
	}

	// ── Shared folders (VirtIO 9P) ────────────────────────────────────────────
	cmdArgs = append(cmdArgs, q.buildSharedFolderArgs(opts.SharedFolders)...)

	return exec.Command(q.executableName, cmdArgs...), nil
}

// buildIfaceArgs returns QEMU args for a single network interface.
func (q *QEMU) buildIfaceArgs(netdevID string, iface NetworkInterface) ([]string, error) {
	macDev := fmt.Sprintf("virtio-net-pci,mac=%s,netdev=%s", iface.MACAddress, netdevID)

	mode := iface.Mode
	if mode == "" {
		mode = NetworkModeNAT
	}

	switch mode {
	case NetworkModeNAT:
		parts := []string{"user", "id=" + netdevID}
		for vmPort, hostPort := range iface.PortForwards {
			parts = append(parts, fmt.Sprintf("hostfwd=tcp:127.0.0.1:%s-:%s", hostPort, vmPort))
		}
		return []string{"-device", macDev, "-netdev", strings.Join(parts, ",")}, nil

	case NetworkModeInternal:
		parts := []string{"user", "id=" + netdevID, "restrict=on"}
		for vmPort, hostPort := range iface.PortForwards {
			parts = append(parts, fmt.Sprintf("hostfwd=tcp:127.0.0.1:%s-:%s", hostPort, vmPort))
		}
		return []string{"-device", macDev, "-netdev", strings.Join(parts, ",")}, nil

	case NetworkModeHostOnly:
		switch runtime.GOOS {
		case "darwin":
			return []string{"-device", macDev, "-netdev", "vmnet-host,id=" + netdevID}, nil
		case "linux":
			parts := []string{
				"user", "id=" + netdevID, "restrict=on",
				"net=192.168.100.0/24", "host=192.168.100.1", "dhcpstart=192.168.100.10",
			}
			for vmPort, hostPort := range iface.PortForwards {
				parts = append(parts, fmt.Sprintf("hostfwd=tcp:127.0.0.1:%s-:%s", hostPort, vmPort))
			}
			return []string{"-device", macDev, "-netdev", strings.Join(parts, ",")}, nil
		}

	case NetworkModeBridged:
		switch runtime.GOOS {
		case "darwin":
			// Requires: sudo or com.apple.vm.networking entitlement.
			return []string{
				"-device", macDev,
				"-netdev", "vmnet-bridged,id=" + netdevID + ",ifname=en0",
			}, nil
		case "linux":
			// Requires a pre-existing tap device on a bridge.
			// Setup: ip tuntap add dev tap0 mode tap && ip link set tap0 master br0 up
			return []string{
				"-device", macDev,
				"-netdev", "tap,id=" + netdevID + ",ifname=tap0,script=no,downscript=no",
			}, nil
		}
	}

	return nil, fmt.Errorf("qemu: unsupported network mode %q on %s", mode, runtime.GOOS)
}

// buildSharedFolderArgs returns QEMU args for each 9P shared folder.
func (q *QEMU) buildSharedFolderArgs(folders []SharedFolder) []string {
	var args []string
	for i, f := range folders {
		fsdevID := fmt.Sprintf("fsdev%d", i)
		args = append(args,
			"-fsdev", fmt.Sprintf("local,id=%s,path=%s,security_model=passthrough", fsdevID, f.HostPath),
			"-device", fmt.Sprintf("virtio-9p-pci,id=fs%d,fsdev=%s,mount_tag=%s", i, fsdevID, f.Tag),
		)
	}
	return args
}

func New(opts NewOptions) (*QEMU, error) {
	qemu := &QEMU{executableName: opts.ExecutableName}

	path, err := exec.LookPath(qemu.executableName)
	if err != nil || path == "" {
		return nil, ErrExecutableNotFound
	}

	switch runtime.GOOS {
	case "darwin":
		qemu.accelerator = "hvf"
	case "linux":
		qemu.accelerator = "kvm"
	default:
		return nil, ErrUnsupportedOperatingSystem
	}

	switch {
	case strings.Contains(qemu.executableName, "aarch64"):
		if runtime.GOARCH != "arm64" {
			return nil, ErrARM64Emulation
		}
		qemu.bios = "edk2-aarch64-code.fd"
		qemu.cpu = "host"
		qemu.machine = "type=virt"

	case strings.Contains(qemu.executableName, "x86_64"):
		if runtime.GOARCH != "amd64" {
			return nil, ErrX8664Emulation
		}
		qemu.cpu = "host"
		qemu.machine = "type=q35"

	default:
		return nil, ErrUnsupportedArchitecture
	}

	return qemu, nil
}
