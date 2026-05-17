package engine

import (
	"encoding/json"
	"fmt"
	"time"
)

// Snapshot represents one internal qcow2 snapshot.
type Snapshot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	VMSize    int64     `json:"vm_state_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type qemuImgInfoOutput struct {
	Snapshots []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		VMStateSize int64  `json:"vm-state-size"`
		DateSec     int64  `json:"date-sec"`
	} `json:"snapshots"`
}

// ListSnapshots returns all snapshots stored in the VM's qcow2 disk.
func (v *VirtualMachine) ListSnapshots() ([]Snapshot, error) {
	out, err := runQemuImg("info", "--output=json", v.diskPath())
	if err != nil {
		return nil, fmt.Errorf("qemu-img info: %w", err)
	}

	var info qemuImgInfoOutput
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parse qemu-img info: %w", err)
	}

	snaps := make([]Snapshot, len(info.Snapshots))
	for i, s := range info.Snapshots {
		snaps[i] = Snapshot{
			ID:        s.ID,
			Name:      s.Name,
			VMSize:    s.VMStateSize,
			CreatedAt: time.Unix(s.DateSec, 0),
		}
	}
	return snaps, nil
}

// CreateSnapshot creates a named qcow2 internal snapshot. The VM must be stopped.
// If name is empty a timestamp is used: "snapshot-2006-01-02T150405".
func (v *VirtualMachine) CreateSnapshot(name string) error {
	if err := v.requireStopped(); err != nil {
		if err == ErrVirtualMachineAlreadyRunning {
			return ErrSnapshotRequiresStopped
		}
		return err
	}

	if name == "" {
		name = "snapshot-" + time.Now().Format("2006-01-02T150405")
	}

	v.engine.printf("Creating snapshot %q on %q\n", name, v.Name)

	out, err := runQemuImg("snapshot", "-c", name, v.diskPath())
	if err != nil {
		return fmt.Errorf("qemu-img snapshot -c %q: %w\n%s", name, err, out)
	}
	return nil
}

// RestoreSnapshot restores a named snapshot. The VM must be stopped.
func (v *VirtualMachine) RestoreSnapshot(name string) error {
	if err := v.requireStopped(); err != nil {
		if err == ErrVirtualMachineAlreadyRunning {
			return ErrSnapshotRequiresStopped
		}
		return err
	}

	// Verify it exists before attempting restore.
	snaps, err := v.ListSnapshots()
	if err != nil {
		return err
	}
	found := false
	for _, s := range snaps {
		if s.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: %q", ErrSnapshotNotFound, name)
	}

	v.engine.printf("Restoring %q to snapshot %q\n", v.Name, name)

	out, err := runQemuImg("snapshot", "-a", name, v.diskPath())
	if err != nil {
		return fmt.Errorf("qemu-img snapshot -a %q: %w\n%s", name, err, out)
	}
	return nil
}

// DeleteSnapshot deletes a named snapshot. The VM must be stopped.
func (v *VirtualMachine) DeleteSnapshot(name string) error {
	if err := v.requireStopped(); err != nil {
		if err == ErrVirtualMachineAlreadyRunning {
			return ErrSnapshotRequiresStopped
		}
		return err
	}

	v.engine.printf("Deleting snapshot %q from %q\n", name, v.Name)

	out, err := runQemuImg("snapshot", "-d", name, v.diskPath())
	if err != nil {
		return fmt.Errorf("qemu-img snapshot -d %q: %w\n%s", name, err, out)
	}
	return nil
}
