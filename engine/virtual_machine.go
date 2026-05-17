package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Igr1s-red/spinup/qemu"
	goxssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// validName restricts VM and profile names to filesystem-safe characters.
var validName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ── Config types ──────────────────────────────────────────────────────────────

// NetworkInterfaceConfig persists one NIC in config.json.
type NetworkInterfaceConfig struct {
	MACAddress   string            `json:"mac_address"`
	NetworkMode  string            `json:"network_mode"`
	PortForwards map[string]string `json:"port_forwards,omitempty"`
}

// SharedFolder is a host directory exposed to the VM via VirtIO 9P.
// Guest mount: mount -t 9p -o trans=virtio,version=9p2000.L <tag> /mnt/pt
type SharedFolder struct {
	HostPath string `json:"host_path"`
	Tag      string `json:"tag"`
}

// VirtualMachineConfig is persisted as config.json alongside the VM data.
// All JSON keys use snake_case for consistency.
type VirtualMachineConfig struct {
	CPU           int                      `json:"cpu"`
	Memory        int                      `json:"memory_mib"`
	DiskSize      int                      `json:"disk_size_gb"`
	Image         string                   `json:"image"`
	SSHUser       string                   `json:"ssh_user"`
	Networks      []NetworkInterfaceConfig `json:"networks"`
	SharedFolders []SharedFolder           `json:"shared_folders,omitempty"`
	// Ephemeral VMs remove themselves when stopped.
	Ephemeral bool `json:"ephemeral,omitempty"`
}

// sshPort returns the host port forwarded to VM port 22, or "".
func (c *VirtualMachineConfig) sshPort() string {
	if len(c.Networks) == 0 || c.Networks[0].PortForwards == nil {
		return ""
	}
	return c.Networks[0].PortForwards["22"]
}

// ── Status ────────────────────────────────────────────────────────────────────

// VirtualMachineStatus is the observable state of a VM process.
type VirtualMachineStatus string

const (
	VirtualMachineStatusStopped VirtualMachineStatus = "stopped"
	VirtualMachineStatusRunning VirtualMachineStatus = "running"
)

// SSHConnectionDetails holds everything needed to reach the VM via SSH.
type SSHConnectionDetails struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	PrivateKey string `json:"private_key"`
}

// VirtualMachineInfo is returned by Inspect().
type VirtualMachineInfo struct {
	Name       string                `json:"name"`
	Status     VirtualMachineStatus  `json:"status"`
	Config     VirtualMachineConfig  `json:"config"`
	SSHDetails *SSHConnectionDetails `json:"ssh,omitempty"`
	LogPath    string                `json:"log_path"`
	DiskPath   string                `json:"disk_path"`
}

// ── VM struct ─────────────────────────────────────────────────────────────────

// VirtualMachine is a configured, named QEMU virtual machine.
type VirtualMachine struct {
	Name   string
	Config VirtualMachineConfig

	engine *Engine
	path   string // absolute path to the VM's data directory
}

// ── Path helpers ──────────────────────────────────────────────────────────────

func (v *VirtualMachine) pidPath() string           { return filepath.Join(v.path, "pid") }
func (v *VirtualMachine) diskPath() string          { return filepath.Join(v.path, "disk.qcow2") }
func (v *VirtualMachine) cloudInitPath() string     { return filepath.Join(v.path, "cloud-init.iso") }
func (v *VirtualMachine) configPath() string        { return filepath.Join(v.path, "config.json") }
func (v *VirtualMachine) privateKeyPath() string    { return filepath.Join(v.path, "key.pem") }
func (v *VirtualMachine) publicKeyPath() string     { return filepath.Join(v.path, "key.pub") }
func (v *VirtualMachine) logPath() string           { return filepath.Join(v.path, "qemu.log") }
func (v *VirtualMachine) monitorSocketPath() string { return filepath.Join(v.path, "monitor.sock") }
func (v *VirtualMachine) consoleSocketPath() string { return filepath.Join(v.path, "console.sock") }

// writeFile creates v.path if needed and writes data with the given permission.
// Pass 0644 for normal files, 0600 for private keys.
func (v *VirtualMachine) writeFile(name string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(v.path, 0755); err != nil {
		return fmt.Errorf("create VM directory: %w", err)
	}
	return os.WriteFile(name, data, perm)
}

// ── Config persistence ────────────────────────────────────────────────────────

// writeConfigFile writes config.json atomically via a temp file + rename.
// This prevents a crash mid-write from leaving a corrupt config on disk.
func (v *VirtualMachine) writeConfigFile() error {
	b, err := json.MarshalIndent(v.Config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmp := v.configPath() + ".tmp"
	if err := v.writeFile(tmp, b, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, v.configPath()); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("commit config: %w", err)
	}
	return nil
}

func (v *VirtualMachine) loadConfigFile() error {
	b, err := os.ReadFile(v.configPath())
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(b, &v.Config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

// ── Process helpers ───────────────────────────────────────────────────────────

func (v *VirtualMachine) findProcess() (*os.Process, error) {
	b, err := os.ReadFile(v.pidPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return nil, fmt.Errorf("parse pid: %w", err)
	}
	return os.FindProcess(pid)
}

// ── Status ────────────────────────────────────────────────────────────────────

// Status reports whether the QEMU process is alive.
func (v *VirtualMachine) Status() (VirtualMachineStatus, error) {
	proc, err := v.findProcess()
	if err != nil {
		return VirtualMachineStatusStopped, err
	}
	if proc == nil {
		return VirtualMachineStatusStopped, nil
	}
	// Signal 0 checks existence without delivering an actual signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return VirtualMachineStatusStopped, nil
	}
	return VirtualMachineStatusRunning, nil
}

// requireStopped returns ErrVirtualMachineAlreadyRunning if the VM is running.
func (v *VirtualMachine) requireStopped() error {
	s, err := v.Status()
	if err != nil {
		return err
	}
	if s == VirtualMachineStatusRunning {
		return ErrVirtualMachineAlreadyRunning
	}
	return nil
}

// requireRunning returns ErrVirtualMachineNotRunning if the VM is not running.
func (v *VirtualMachine) requireRunning() error {
	s, err := v.Status()
	if err != nil {
		return err
	}
	if s != VirtualMachineStatusRunning {
		return ErrVirtualMachineNotRunning
	}
	return nil
}

// isEphemeral reports whether the VM should remove itself when stopped.
// Defined here; vm_autostart.go must not redeclare it.
func (v *VirtualMachine) isEphemeral() bool { return v.Config.Ephemeral }

// ── Start ─────────────────────────────────────────────────────────────────────

// StartOptions allows per-run resource overrides without changing stored config.
type StartOptions struct {
	CPU    int // > 0 overrides stored value for this run only
	Memory int // > 0 overrides stored value (MiB) for this run only
}

// Start starts the VM with its stored configuration.
func (v *VirtualMachine) Start() error { return v.StartWithOptions(StartOptions{}) }

// StartWithOptions starts the VM with optional per-run overrides.
// If QEMU cannot bind the SSH port (TOCTOU race), it retries up to 3 times
// with a freshly allocated port.
func (v *VirtualMachine) StartWithOptions(opts StartOptions) error {
	if err := v.requireStopped(); err != nil {
		return err
	}

	cpu := v.Config.CPU
	if opts.CPU > 0 {
		cpu = opts.CPU
		v.engine.printf("CPU override: %d\n", cpu)
	}

	memory := v.Config.Memory
	if opts.Memory > 0 {
		memory = opts.Memory
		v.engine.printf("Memory override: %d MiB\n", memory)
	}

	v.engine.printf("Starting VM %q (cpu=%d mem=%dMiB)\n", v.Name, cpu, memory)

	// Pre-build shared folder list — constant across retries.
	qFolders := make([]qemu.SharedFolder, len(v.Config.SharedFolders))
	for i, sf := range v.Config.SharedFolders {
		qFolders[i] = qemu.SharedFolder{HostPath: sf.HostPath, Tag: sf.Tag}
	}

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Rebuild NICs each attempt so a reallocated port is picked up.
		ifaces := make([]qemu.NetworkInterface, len(v.Config.Networks))
		for i, n := range v.Config.Networks {
			ifaces[i] = qemu.NetworkInterface{
				MACAddress:   n.MACAddress,
				Mode:         qemu.NetworkMode(n.NetworkMode),
				PortForwards: n.PortForwards,
			}
		}

		cmd, err := v.engine.qemu.Command(qemu.CommandOptions{
			CPU:               cpu,
			Memory:            memory,
			Disks:             []qemu.CommandOptionsDisk{{Path: v.diskPath()}, {Path: v.cloudInitPath(), ReadOnly: true}},
			Interfaces:        ifaces,
			SharedFolders:     qFolders,
			MonitorSocketPath: v.monitorSocketPath(),
			ConsoleSocketPath: v.consoleSocketPath(),
		})
		if err != nil {
			return fmt.Errorf("build QEMU command: %w", err)
		}

		v.engine.printf("Command: %s\n", strings.Join(cmd.Args, " "))

		logFile, err := os.OpenFile(v.logPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		startErr := cmd.Start()
		logFile.Close()

		if startErr == nil {
			return v.writeFile(v.pidPath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		}

		if attempt < maxRetries && isPortInUse(startErr) {
			v.engine.printf("SSH port conflict (attempt %d) — reassigning...\n", attempt)
			if err := v.reassignSSHPort(); err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("start QEMU: %w", startErr)
	}
	return fmt.Errorf("start QEMU: port conflict persists after %d attempts", maxRetries)
}

func (v *VirtualMachine) reassignSSHPort() error {
	// Use 127.0.0.1:0 to guarantee a loopback address is returned.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("allocate free port: %w", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	if len(v.Config.Networks) == 0 {
		return fmt.Errorf("no network interfaces configured")
	}
	if v.Config.Networks[0].PortForwards == nil {
		v.Config.Networks[0].PortForwards = map[string]string{}
	}
	v.Config.Networks[0].PortForwards["22"] = strconv.Itoa(port)
	return v.writeConfigFile()
}

func isPortInUse(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "address already in use") ||
		strings.Contains(s, "bind: can't assign requested address")
}

// ── Stop ──────────────────────────────────────────────────────────────────────

// Stop shuts down the VM. It attempts graceful ACPI shutdown via QMP first,
// waiting up to 15 s before falling back to SIGKILL.
func (v *VirtualMachine) Stop() error {
	if err := v.requireRunning(); err != nil {
		return err
	}

	v.engine.printf("Stopping VM %q\n", v.Name)

	// Graceful path: ACPI power-down via QMP.
	if qmp, err := qemu.NewQMPClient(v.monitorSocketPath()); err == nil {
		if powerErr := qmp.SystemPowerdown(); powerErr == nil {
			qmp.Close()
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(500 * time.Millisecond)
				if s, _ := v.Status(); s == VirtualMachineStatusStopped {
					os.Remove(v.pidPath())
					return v.postStop()
				}
			}
			v.engine.printf("Graceful shutdown timed out — killing\n")
		} else {
			qmp.Close()
		}
	}

	// Fallback: SIGKILL.
	proc, err := v.findProcess()
	if err != nil {
		return err
	}
	if proc == nil {
		return ErrVirtualMachineNotRunning
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("kill: %w", err)
	}
	if err := os.Remove(v.pidPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pid: %w", err)
	}
	return v.postStop()
}

// postStop runs cleanup after the VM process exits.
func (v *VirtualMachine) postStop() error {
	if v.isEphemeral() {
		v.engine.printf("VM %q is ephemeral — removing\n", v.Name)
		return v.Remove()
	}
	return nil
}

// ── Remove ────────────────────────────────────────────────────────────────────

// Remove stops the VM if running, then deletes all its data.
func (v *VirtualMachine) Remove() error {
	if s, err := v.Status(); err != nil {
		return err
	} else if s == VirtualMachineStatusRunning {
		if err := v.Stop(); err != nil {
			return err
		}
	}
	// Best-effort: clean up SSH config before the data directory is gone.
	if err := v.SSHConfigRemove(); err != nil {
		v.engine.printf("Warning: could not remove SSH config entry: %s\n", err)
	}
	v.engine.printf("Removing VM %q\n", v.Name)
	if err := os.RemoveAll(v.path); err != nil {
		return fmt.Errorf("remove VM data: %w", err)
	}
	delete(v.engine.virtualMachines, v.Name)
	return nil
}

// ── SSH helpers ───────────────────────────────────────────────────────────────

func (v *VirtualMachine) sshClient() (*goxssh.Client, error) {
	port := v.Config.sshPort()
	if port == "" {
		return nil, ErrInvalidSSHPort
	}

	keyBytes, err := os.ReadFile(v.privateKeyPath())
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	signer, err := goxssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return goxssh.Dial("tcp", "127.0.0.1:"+port, &goxssh.ClientConfig{
		User:            v.Config.SSHUser,
		Auth:            []goxssh.AuthMethod{goxssh.PublicKeys(signer)},
		HostKeyCallback: goxssh.InsecureIgnoreHostKey(), //nolint:gosec
		BannerCallback:  goxssh.BannerDisplayStderr(),
		Timeout:         10 * time.Second,
	})
}

// SSHSessionWithXterm opens an interactive SSH session with a PTY.
func (v *VirtualMachine) SSHSessionWithXterm() error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	v.engine.printf("Connecting to %q via SSH\n", v.Name)

	client, err := v.sshClient()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdin, session.Stdout, session.Stderr = os.Stdin, os.Stdout, os.Stderr

	w, h, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 24 // safe fallback if not a real terminal
	}
	if err := session.RequestPty("xterm-256color", h, w, goxssh.TerminalModes{}); err != nil {
		return fmt.Errorf("request PTY: %w", err)
	}
	if err := session.Shell(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	return session.Wait()
}

// Exec runs a single command inside the VM over SSH.
func (v *VirtualMachine) Exec(command string) error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	client, err := v.sshClient()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdout, session.Stderr = os.Stdout, os.Stderr
	return session.Run(command)
}

// SSHConnectionDetails returns SSH connection info. VM must be running.
func (v *VirtualMachine) SSHConnectionDetails() (*SSHConnectionDetails, error) {
	if err := v.requireRunning(); err != nil {
		return nil, err
	}
	port := v.Config.sshPort()
	if port == "" {
		return nil, ErrInvalidSSHPort
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		return nil, ErrInvalidSSHPort
	}
	return &SSHConnectionDetails{
		Host:       "127.0.0.1",
		Port:       portI,
		Username:   v.Config.SSHUser,
		PrivateKey: v.privateKeyPath(),
	}, nil
}

// scp runs an scp command with the VM's stored key and port.
func (v *VirtualMachine) scp(src, dst string) error {
	port := v.Config.sshPort()
	if port == "" {
		return ErrInvalidSSHPort
	}
	cmd := scpCmd(v.privateKeyPath(), port, src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scp %s → %s: %w", src, dst, err)
	}
	return nil
}

// CopyFrom downloads a file from the VM to the local filesystem.
func (v *VirtualMachine) CopyFrom(remotePath, localPath string) error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	return v.scp(fmt.Sprintf("%s@127.0.0.1:%s", v.Config.SSHUser, remotePath), localPath)
}

// CopyTo uploads a local file into the VM.
func (v *VirtualMachine) CopyTo(localPath, remotePath string) error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	return v.scp(localPath, fmt.Sprintf("%s@127.0.0.1:%s", v.Config.SSHUser, remotePath))
}

// ── Logs ──────────────────────────────────────────────────────────────────────

// Logs streams the QEMU console log to stdout.
// When follow is true it behaves like tail -F: it polls every 250 ms and
// handles log rotation caused by VM restarts (O_TRUNC on the same path).
// ctx cancellation stops the follow loop cleanly.
func (v *VirtualMachine) Logs(ctx context.Context, follow bool) error {
	logPath := v.logPath()

	f, err := os.Open(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return ErrVirtualMachineLogNotFound
	}
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}

	if _, err := io.Copy(os.Stdout, f); err != nil {
		f.Close()
		return fmt.Errorf("stream log: %w", err)
	}
	if !follow {
		f.Close()
		return nil
	}

	fi, _ := f.Stat()
	buf := make([]byte, 32*1024)

	// reopen closes the current file and opens a fresh handle to logPath,
	// updating f and fi in the enclosing scope.
	reopen := func() error {
		f.Close()
		var openErr error
		f, openErr = os.Open(logPath)
		if openErr != nil {
			return fmt.Errorf("reopen log: %w", openErr)
		}
		fi, _ = f.Stat()
		return nil
	}

	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, werr := os.Stdout.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
		}

		if readErr == io.EOF {
			// Detect log rotation: O_TRUNC (same inode, smaller size) or
			// delete+recreate (different inode).
			if diskFi, statErr := os.Stat(logPath); statErr == nil {
				if !os.SameFile(fi, diskFi) {
					if err := reopen(); err != nil {
						return err
					}
				} else {
					curPos, _ := f.Seek(0, io.SeekCurrent)
					if diskFi.Size() < curPos {
						f.Seek(0, io.SeekStart) //nolint:errcheck
						fi = diskFi
					}
				}
			}

			select {
			case <-ctx.Done():
				f.Close()
				return nil
			case <-time.After(250 * time.Millisecond):
			}
			continue
		}

		if readErr != nil {
			f.Close()
			return fmt.Errorf("read log: %w", readErr)
		}

		select {
		case <-ctx.Done():
			f.Close()
			return nil
		default:
		}
	}
}

// ── Inspect ───────────────────────────────────────────────────────────────────

// Inspect returns a full snapshot of the VM's state as a JSON-serialisable struct.
func (v *VirtualMachine) Inspect() (*VirtualMachineInfo, error) {
	status, err := v.Status()
	if err != nil {
		return nil, err
	}

	info := &VirtualMachineInfo{
		Name:     v.Name,
		Status:   status,
		Config:   v.Config,
		LogPath:  v.logPath(),
		DiskPath: v.diskPath(),
	}
	if status == VirtualMachineStatusRunning {
		if d, err := v.SSHConnectionDetails(); err == nil {
			info.SSHDetails = d
		}
	}
	return info, nil
}

// ── Port-forward management ───────────────────────────────────────────────────

// AddPortForward adds a host→VM port forward to the primary NIC.
// The VM must be stopped; takes effect on next start.
func (v *VirtualMachine) AddPortForward(hostPort, vmPort string) error {
	if err := v.requireStopped(); err != nil {
		if err == ErrVirtualMachineAlreadyRunning {
			return ErrPortForwardVMRunning
		}
		return err
	}
	if len(v.Config.Networks) == 0 {
		return fmt.Errorf("no network interfaces configured")
	}
	if v.Config.Networks[0].PortForwards == nil {
		v.Config.Networks[0].PortForwards = map[string]string{}
	}
	v.Config.Networks[0].PortForwards[vmPort] = hostPort
	return v.writeConfigFile()
}

// RemovePortForward removes a forward by VM port from the primary NIC.
// The VM must be stopped.
func (v *VirtualMachine) RemovePortForward(vmPort string) error {
	if err := v.requireStopped(); err != nil {
		if err == ErrVirtualMachineAlreadyRunning {
			return ErrPortForwardVMRunning
		}
		return err
	}
	if len(v.Config.Networks) == 0 {
		return ErrPortForwardNotFound
	}
	pf := v.Config.Networks[0].PortForwards
	if _, ok := pf[vmPort]; !ok {
		return ErrPortForwardNotFound
	}
	delete(pf, vmPort)
	return v.writeConfigFile()
}


// ── CreateVirtualMachine ──────────────────────────────────────────────────────

// cloudInitTemplate is the default cloud-init user-data. It disables SSH
// password auth (key-only) and leaves a console password for emergency access.
// %s is replaced with the VM's SSH authorized key line.
const cloudInitTemplate = "#cloud-config\n\n" +
	"password: password\n" +
	"chpasswd: { expire: False }\n" +
	"ssh_pwauth: False\n" +
	"ssh_authorized_keys:\n" +
	"  - %s"

// cloudInitNetworkConfig enables DHCP on any enp* interface (networkd v2).
const cloudInitNetworkConfig = `version: 2
ethernets:
  interface0:
    match:
      name: enp*
    dhcp4: true
    dhcp6: true
`

func (e *Engine) CreateVirtualMachine(opts CreateVirtualMachineOptions) (*VirtualMachine, error) {
	if opts.Name == "" || !validName.MatchString(opts.Name) {
		return nil, ErrInvalidName
	}
	if opts.CPU < 1 {
		return nil, ErrInvalidCPU
	}
	if opts.Memory < 128 {
		return nil, ErrInvalidMemory
	}
	if opts.DiskSize < 1 {
		return nil, ErrInvalidDisk
	}
	if e.FindVirtualMachine(opts.Name) != nil {
		return nil, ErrVirtualMachineAlreadyExist
	}

	img := e.FindImage(opts.Image)
	if img == nil {
		return nil, fmt.Errorf("%w: %q", ErrImageNotFound, opts.Image)
	}

	if pulled, err := img.Pulled(); err != nil {
		return nil, err
	} else if !pulled {
		e.printf("Image %q not found locally — pulling\n", opts.Image)
		if err := img.Pull(); err != nil {
			return nil, err
		}
	}

	e.printf("Creating VM %q from %q\n", opts.Name, opts.Image)

	// ── NICs ──────────────────────────────────────────────────────────────────
	networks := opts.Networks
	if len(networks) == 0 {
		networks = []NetworkInterfaceConfig{{NetworkMode: "nat"}}
	}
	for i := range networks {
		if networks[i].MACAddress == "" {
			mac, err := RandomMAC()
			if err != nil {
				return nil, err
			}
			networks[i].MACAddress = mac
			e.printf("NIC %d MAC: %s\n", i, mac)
		}
		if networks[i].PortForwards == nil {
			networks[i].PortForwards = map[string]string{}
		}
	}
	if networks[0].PortForwards["22"] == "" {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("allocate SSH port: %w", err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		networks[0].PortForwards["22"] = strconv.Itoa(port)
		e.printf("SSH port: %s\n", networks[0].PortForwards["22"])
	}

	// ── SSH key (Ed25519) ─────────────────────────────────────────────────────
	e.printf("Generating Ed25519 SSH key\n")

	vm := &VirtualMachine{
		Name:   opts.Name,
		engine: e,
		path:   e.virtualMachinePath(opts.Name),
		Config: VirtualMachineConfig{
			CPU:           opts.CPU,
			Memory:        opts.Memory,
			Image:         opts.Image,
			SSHUser:       img.sshUser,
			DiskSize:      opts.DiskSize,
			Networks:      networks,
			SharedFolders: opts.SharedFolders,
			Ephemeral:     opts.Ephemeral,
		},
	}

	pubKeyBytes, err := vm.generateSSHKey()
	if err != nil {
		return nil, err
	}

	// ── Cloud-init ISO ────────────────────────────────────────────────────────
	e.printf("Building cloud-init ISO\n")
	userData := opts.UserData
	if userData == "" {
		userData = defaultCloudInitUserData(pubKeyBytes)
	}
	if err := vm.buildCloudInit(userData, vm.Name); err != nil {
		return nil, fmt.Errorf("build cloud-init: %w", err)
	}

	// ── Disk ──────────────────────────────────────────────────────────────────
	e.printf("Copying image disk\n")
	if err := copyFileTo(img.diskPath(), vm.diskPath()); err != nil {
		return nil, err
	}

	e.printf("Resizing disk to %d GB\n", opts.DiskSize)
	if out, err := runQemuImg("resize", vm.diskPath(), fmt.Sprintf("%dG", opts.DiskSize)); err != nil {
		return nil, fmt.Errorf("qemu-img resize: %w\n%s", err, out)
	}

	if err := vm.writeConfigFile(); err != nil {
		return nil, err
	}

	e.virtualMachines[opts.Name] = vm
	return vm, nil
}
