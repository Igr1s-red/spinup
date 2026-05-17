package engine

import (
	"fmt"
	"strings"
)

// VMStats holds resource usage data for a VM.
// Config fields are always populated. Live fields require the VM to be
// running with the QMP monitor available.
type VMStats struct {
	Name       string `json:"name"`
	CPUCount   int    `json:"cpu_count"`
	MemoryMiB  int    `json:"memory_mib"`
	DiskSizeGB int    `json:"disk_size_gb"`

	QMPAvailable     bool        `json:"qmp_available"`
	BalloonMemoryMiB int64       `json:"balloon_memory_mib,omitempty"`
	BlockDevices     []BlockStat `json:"block_devices,omitempty"`
}

// BlockStat holds I/O counters for one block device.
type BlockStat struct {
	Device     string `json:"device"`
	Filename   string `json:"filename,omitempty"`
	Format     string `json:"format,omitempty"`
	ReadBytes  int64  `json:"read_bytes"`
	WriteBytes int64  `json:"write_bytes"`
	ReadOps    int64  `json:"read_ops"`
	WriteOps   int64  `json:"write_ops"`
}

// Stats returns resource usage for the VM.
// Config values are always available; live I/O stats require a running VM.
func (v *VirtualMachine) Stats() (*VMStats, error) {
	s := &VMStats{
		Name:       v.Name,
		CPUCount:   v.Config.CPU,
		MemoryMiB:  v.Config.Memory,
		DiskSizeGB: v.Config.DiskSize,
	}

	status, err := v.Status()
	if err != nil {
		return nil, err
	}
	if status != VirtualMachineStatusRunning {
		return s, nil
	}

	qmp, err := v.connectQMP()
	if err != nil {
		// QMP not yet ready — return config stats only, not an error.
		return s, nil
	}
	defer qmp.Close()
	s.QMPAvailable = true

	if balloon, err := qmp.QueryBalloon(); err == nil {
		s.BalloonMemoryMiB = balloon.Actual / (1024 * 1024)
	}

	if devs, err := qmp.QueryBlock(); err == nil {
		for _, d := range devs {
			if d.Device == "" || d.Inserted == nil {
				continue
			}
			s.BlockDevices = append(s.BlockDevices, BlockStat{
				Device:     d.Device,
				Filename:   d.Inserted.Image.Filename,
				Format:     d.Inserted.Image.Format,
				ReadBytes:  d.Inserted.RdBytes,
				WriteBytes: d.Inserted.WrBytes,
				ReadOps:    d.Inserted.RdOps,
				WriteOps:   d.Inserted.WrOps,
			})
		}
	}

	return s, nil
}

// FormatStats returns a human-readable stats summary.
func FormatStats(s *VMStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "VM:      %s\n", s.Name)
	fmt.Fprintf(&b, "CPU:     %d vCPU(s)\n", s.CPUCount)
	fmt.Fprintf(&b, "Memory:  %d MiB (configured)\n", s.MemoryMiB)
	fmt.Fprintf(&b, "Disk:    %d GB\n", s.DiskSizeGB)

	if !s.QMPAvailable {
		fmt.Fprintf(&b, "Live stats: unavailable (VM not running or QMP not ready)\n")
		return b.String()
	}

	if s.BalloonMemoryMiB > 0 {
		fmt.Fprintf(&b, "Balloon: %d MiB (current target)\n", s.BalloonMemoryMiB)
	}

	if len(s.BlockDevices) > 0 {
		fmt.Fprintf(&b, "Block I/O:\n")
		for _, bd := range s.BlockDevices {
			fmt.Fprintf(&b, "  %-14s  read: %s (%d ops)  write: %s (%d ops)\n",
				bd.Device,
				formatBytes(bd.ReadBytes), bd.ReadOps,
				formatBytes(bd.WriteBytes), bd.WriteOps,
			)
		}
	}

	return b.String()
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
