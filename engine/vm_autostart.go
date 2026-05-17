package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

// AutostartEnable configures the VM to start automatically when the user logs in.
//
// On macOS a launchd plist is written to ~/Library/LaunchAgents/.
// On Linux a systemd user unit is written to ~/.config/systemd/user/.
func (v *VirtualMachine) AutostartEnable() error {
	switch runtime.GOOS {
	case "darwin":
		return v.autostartEnableMacOS()
	case "linux":
		return v.autostartEnableLinux()
	default:
		return fmt.Errorf("autostart is not supported on %s", runtime.GOOS)
	}
}

// AutostartDisable removes the autostart configuration for the VM.
func (v *VirtualMachine) AutostartDisable() error {
	switch runtime.GOOS {
	case "darwin":
		return v.autostartDisableMacOS()
	case "linux":
		return v.autostartDisableLinux()
	default:
		return fmt.Errorf("autostart is not supported on %s", runtime.GOOS)
	}
}

// AutostartEnabled reports whether autostart is currently configured.
func (v *VirtualMachine) AutostartEnabled() (bool, error) {
	path, err := v.autostartPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// ── macOS ─────────────────────────────────────────────────────────────────────

const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.SpinupPath}}</string>
        <string>start</string>
        <string>{{.VMName}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>
`

type launchdVars struct {
	Label      string
	SpinupPath string
	VMName     string
	LogPath    string
}

func (v *VirtualMachine) launchdLabel() string {
	return fmt.Sprintf("com.spinup.vm.%s", v.Name)
}

func (v *VirtualMachine) autostartPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "LaunchAgents",
			fmt.Sprintf("%s.plist", v.launchdLabel())), nil
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "systemd", "user",
			fmt.Sprintf("spinup-%s.service", v.Name)), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func (v *VirtualMachine) autostartEnableMacOS() error {
	plistPath, err := v.autostartPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(plistPath); err == nil {
		return ErrAutoStartExists
	}

	spinupPath, err := findSelf()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}

	tmpl, err := template.New("plist").Parse(launchdTemplate)
	if err != nil {
		return fmt.Errorf("parse plist template: %w", err)
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, launchdVars{
		Label:      v.launchdLabel(),
		SpinupPath: spinupPath,
		VMName:     v.Name,
		LogPath:    v.logPath(),
	}); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	// Load the plist immediately so it takes effect without logout.
	if out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
		v.engine.printf("Warning: launchctl load failed: %s\n%s\n", err, out)
	}

	v.engine.printf("Autostart enabled → %s\n", plistPath)
	v.engine.printf("%q will start automatically at login\n", v.Name)
	return nil
}

func (v *VirtualMachine) autostartDisableMacOS() error {
	plistPath, err := v.autostartPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return ErrAutoStartMissing
	}

	// Unload before removing so launchd forgets about it.
	exec.Command("launchctl", "unload", plistPath).Run() //nolint:errcheck

	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("remove plist: %w", err)
	}

	v.engine.printf("Autostart disabled for %q\n", v.Name)
	return nil
}

// ── Linux (systemd user) ──────────────────────────────────────────────────────

const systemdTemplate = `[Unit]
Description=spinup VM: {{.VMName}}
After=network.target

[Service]
Type=forking
ExecStart={{.SpinupPath}} start {{.VMName}}
ExecStop={{.SpinupPath}} stop {{.VMName}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

type systemdVars struct {
	VMName     string
	SpinupPath string
}

func (v *VirtualMachine) autostartEnableLinux() error {
	unitPath, err := v.autostartPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(unitPath); err == nil {
		return ErrAutoStartExists
	}

	spinupPath, err := findSelf()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	tmpl, err := template.New("unit").Parse(systemdTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("create unit file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, systemdVars{
		VMName:     v.Name,
		SpinupPath: spinupPath,
	}); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}

	unitName := fmt.Sprintf("spinup-%s.service", v.Name)

	// Reload and enable.
	exec.Command("systemctl", "--user", "daemon-reload").Run() //nolint:errcheck
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", unitName).CombinedOutput(); err != nil {
		v.engine.printf("Warning: systemctl enable failed: %s\n%s\n", err, out)
	}

	v.engine.printf("Autostart enabled → %s\n", unitPath)
	v.engine.printf("%q will start automatically at login\n", v.Name)
	return nil
}

func (v *VirtualMachine) autostartDisableLinux() error {
	unitPath, err := v.autostartPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		return ErrAutoStartMissing
	}

	unitName := fmt.Sprintf("spinup-%s.service", v.Name)
	exec.Command("systemctl", "--user", "disable", "--now", unitName).Run() //nolint:errcheck

	if err := os.Remove(unitPath); err != nil {
		return fmt.Errorf("remove unit: %w", err)
	}

	v.engine.printf("Autostart disabled for %q\n", v.Name)
	return nil
}

// findSelf returns the absolute path of the currently running spinup binary.
// Used to write the correct path into launchd/systemd files.
func findSelf() (string, error) {
	// Try os.Executable first (most reliable).
	if exe, err := os.Executable(); err == nil {
		return exe, nil
	}

	// Fall back to searching PATH.
	if path, err := exec.LookPath("spinup"); err == nil {
		return path, nil
	}

	// Check common install locations.
	for _, candidate := range []string{
		"/usr/local/bin/spinup",
		"/opt/homebrew/bin/spinup",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("cannot locate spinup binary — set PATH or reinstall")
}

// Ephemeral marks the VM to remove itself when stopped.
func (v *VirtualMachine) markEphemeral() {
	v.Config.Ephemeral = true
}

