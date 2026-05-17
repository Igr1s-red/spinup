package engine

import (
	"fmt"
)

// Resize grows the VM's disk to newSizeGB gigabytes.
// The VM must be stopped. Shrinking is intentionally unsupported — it
// requires in-guest filesystem work and risks data loss.
// After resizing, boot the VM and run the appropriate tool inside the guest
// to expand the filesystem (e.g. `growpart /dev/vda 1 && resize2fs /dev/vda1`).
func (v *VirtualMachine) Resize(newSizeGB int) error {
	if newSizeGB <= v.Config.DiskSize {
		return fmt.Errorf(
			"new size (%d GB) must be larger than current size (%d GB) — "+
				"shrinking is not supported",
			newSizeGB, v.Config.DiskSize,
		)
	}

	if err := v.requireStopped(); err != nil {
		return err
	}

	v.engine.printf("Resizing disk from %d GB to %d GB\n", v.Config.DiskSize, newSizeGB)

	out, err := runQemuImg("resize", v.diskPath(), fmt.Sprintf("%dG", newSizeGB))
	if err != nil {
		return fmt.Errorf("qemu-img resize: %w\n%s", err, out)
	}

	v.Config.DiskSize = newSizeGB
	if err := v.writeConfigFile(); err != nil {
		return fmt.Errorf("update config after resize: %w", err)
	}

	v.engine.printf("Disk resized to %d GB. "+
		"Inside the guest run: growpart /dev/vda 1 && resize2fs /dev/vda1\n",
		newSizeGB,
	)
	return nil
}

// Balloon adjusts the running VM's memory balloon target.
// mib is the target memory size in mebibytes.
// Requires the VM to be running and the virtio-balloon-pci device (always present).
func (v *VirtualMachine) Balloon(mib int) error {
	if err := v.requireRunning(); err != nil {
		return err
	}
	if mib < 64 {
		return fmt.Errorf("balloon target must be at least 64 MiB")
	}

	qmp, err := v.connectQMP()
	if err != nil {
		return err
	}
	defer qmp.Close()

	bytes := int64(mib) * 1024 * 1024
	if err := qmp.SetBalloon(bytes); err != nil {
		return fmt.Errorf("set balloon: %w", err)
	}

	v.engine.printf("Balloon target set to %d MiB\n", mib)
	return nil
}
