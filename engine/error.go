package engine

import "errors"

var (
	// Image errors
	ErrImageNotFound   = errors.New("engine: image not found")
	ErrInvalidChecksum = errors.New("engine: checksum mismatch")

	// VM lifecycle
	ErrVirtualMachineAlreadyExist   = errors.New("engine: virtual machine already exists")
	ErrVirtualMachineAlreadyRunning = errors.New("engine: virtual machine is already running")
	ErrVirtualMachineNotRunning     = errors.New("engine: virtual machine is not running")
	ErrVirtualMachineLogNotFound    = errors.New("engine: log not found — start the VM at least once")
	ErrCloneSourceRunning           = errors.New("engine: source VM must be stopped before cloning")

	// SSH / networking
	ErrInvalidSSHPort   = errors.New("engine: no SSH port forward configured")
	ErrSSHTimeout       = errors.New("engine: timed out waiting for SSH to become available")
	ErrSSHAuthTimeout   = errors.New("engine: timed out waiting for SSH key auth to succeed")
	ErrCopyIDNotFound   = errors.New("engine: no ~/.ssh/id_*.pub key found — specify one with --key")
	ErrSshfsNotFound    = errors.New("engine: sshfs not found — install with: brew install macfuse sshfs (macOS) or apt install sshfs (Linux)")
	ErrAutoStartExists  = errors.New("engine: autostart is already enabled for this VM")
	ErrAutoStartMissing = errors.New("engine: autostart is not enabled for this VM")

	// Snapshots
	ErrSnapshotNotFound        = errors.New("engine: snapshot not found")
	ErrSnapshotRequiresStopped = errors.New("engine: snapshots require the VM to be stopped")

	// Port forwards
	ErrPortForwardNotFound  = errors.New("engine: port forward not found")
	ErrPortForwardVMRunning = errors.New("engine: stop the VM before changing port forwards")

	// Validation
	ErrInvalidName   = errors.New("engine: name must be non-empty and use only letters, digits, hyphens, and underscores")
	ErrInvalidCPU    = errors.New("engine: CPU count must be at least 1")
	ErrInvalidMemory = errors.New("engine: memory must be at least 128 MiB")
	ErrInvalidDisk   = errors.New("engine: disk size must be at least 1 GB")

	// QMP / runtime
	ErrQMPNotAvailable         = errors.New("engine: QMP monitor not available — is the VM running?")
	ErrUnsupportedArchitecture = errors.New("engine: unsupported architecture")

	// Disk
	ErrResizeShrinkNotSupported = errors.New("engine: shrinking disks is not supported — only grow is allowed")

	// Profiles
	ErrProfileNotFound      = errors.New("engine: profile not found")
	ErrProfileAlreadyExists = errors.New("engine: profile already exists")

	// Balloon
	ErrInvalidBalloonTarget = errors.New("engine: balloon target must be at least 64 MiB")

	// Ephemeral
	ErrNotEphemeral = errors.New("engine: VM is not ephemeral")

	// Console
	ErrConsoleNotAvailable = errors.New("engine: console socket not available — start the VM first (VMs started before console support was added need a restart)")
	ErrSocatNotFound       = errors.New("engine: socat not found — install with: apt install socat (Linux) or brew install socat (macOS)")
)
