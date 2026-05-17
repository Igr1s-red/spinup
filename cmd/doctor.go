package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the environment for configuration problems",
		Long: `doctor verifies that all tools spinup depends on are installed and accessible.
Run this first when you encounter unexpected errors.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			runDoctor(globalOptions)
			return nil
		},
	}
}

type doctorResult struct {
	label  string
	status string // "ok" | "warn" | "fail"
	detail string
}

func runDoctor(opts *globalOptions) {
	var results []doctorResult

	add := func(label, status, detail string) {
		results = append(results, doctorResult{label, status, detail})
	}

	// ── QEMU binary ───────────────────────────────────────────────────────────
	if path, err := exec.LookPath(opts.qemuExecutableName); err == nil {
		add("QEMU ("+opts.qemuExecutableName+")", "ok", path)
	} else {
		var hint string
		switch runtime.GOOS {
		case "linux":
			hint = "sudo apt install qemu-system-x86  OR  sudo dnf install qemu-kvm"
		case "darwin":
			hint = "brew install qemu"
		default:
			hint = "install QEMU for your OS"
		}
		add("QEMU ("+opts.qemuExecutableName+")", "fail", "not found — "+hint)
	}

	// ── qemu-img ──────────────────────────────────────────────────────────────
	if path, err := exec.LookPath("qemu-img"); err == nil {
		add("qemu-img", "ok", path)
	} else {
		var hint string
		switch runtime.GOOS {
		case "linux":
			hint = "sudo apt install qemu-utils  OR  sudo dnf install qemu-img"
		case "darwin":
			hint = "brew install qemu"
		default:
			hint = "install qemu-img for your OS"
		}
		add("qemu-img", "fail", "not found — "+hint)
	}

	// ── Hardware acceleration ─────────────────────────────────────────────────
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open("/dev/kvm")
		if err == nil {
			f.Close()
			add("KVM acceleration", "ok", "/dev/kvm is accessible")
		} else if os.IsPermission(err) {
			add("KVM acceleration", "warn",
				"/dev/kvm exists but is not accessible — run: sudo usermod -aG kvm $USER  (then log out/in)")
		} else {
			add("KVM acceleration", "warn",
				"/dev/kvm not found — QEMU will fall back to software emulation (slow). Enable KVM in BIOS/kernel.")
		}
	case "darwin":
		add("HVF acceleration", "ok", "available on all Apple Silicon and Intel Macs running macOS 10.10+")
	}

	// ── sshfs (optional — only needed for spinup mount) ───────────────────────
	if path, err := exec.LookPath("sshfs"); err == nil {
		add("sshfs", "ok", path+" (spinup mount available)")
	} else {
		var hint string
		switch runtime.GOOS {
		case "linux":
			hint = "sudo apt install sshfs"
		case "darwin":
			hint = "brew install macfuse sshfs"
		default:
			hint = "install sshfs"
		}
		add("sshfs", "warn", "not found — spinup mount unavailable ("+hint+")")
	}

	// ── Data directory ────────────────────────────────────────────────────────
	if fi, err := os.Stat(opts.configPath); err == nil {
		if fi.IsDir() {
			add("Data directory", "ok", opts.configPath)
		} else {
			add("Data directory", "fail", opts.configPath+" exists but is not a directory")
		}
	} else if os.IsNotExist(err) {
		add("Data directory", "warn", opts.configPath+" does not exist yet — will be created on first use")
	} else {
		add("Data directory", "fail", opts.configPath+": "+err.Error())
	}

	// ── Print results ─────────────────────────────────────────────────────────
	fmt.Println("spinup doctor")
	fmt.Println()

	fails, warns := 0, 0
	for _, r := range results {
		var tag string
		switch r.status {
		case "ok":
			tag = "[ ok ] "
		case "warn":
			tag = "[warn] "
			warns++
		case "fail":
			tag = "[FAIL] "
			fails++
		}
		fmt.Printf("  %s %-36s %s\n", tag, r.label, r.detail)
	}

	fmt.Println()
	switch {
	case fails > 0:
		fmt.Printf("[FAIL] %d required item(s) missing — fix the [FAIL] entries before using spinup.\n", fails)
	case warns > 0:
		fmt.Printf("[ ok ] All required checks passed (%d optional item(s) missing).\n", warns)
	default:
		fmt.Println("[ ok ] All checks passed.")
	}
}
