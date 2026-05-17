package cmd

import (
	"os"
	"path"
	"runtime"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/Igr1s-red/spinup/qemu"
	"github.com/spf13/cobra"
)

type globalOptions struct {
	qemuExecutableName string
	configPath         string
}

func New() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Short:         "Spin up Linux VMs with QEMU",
		Use:           "spinup",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// ── VM lifecycle ──────────────────────────────────────────────────────────
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newStopCommand())
	cmd.AddCommand(newRestartCommand())
	cmd.AddCommand(newRemoveCommand())
	cmd.AddCommand(newRenameCommand())
	cmd.AddCommand(newWaitCommand())
	cmd.AddCommand(newPauseCommand())
	cmd.AddCommand(newResumeCommand())

	// ── VM interaction ────────────────────────────────────────────────────────
	cmd.AddCommand(newSSHCommand())   // opens shell by default
	cmd.AddCommand(newXtermCommand()) // kept as alias for interactive SSH
	cmd.AddCommand(newExecCommand())
	cmd.AddCommand(newCpCommand())
	cmd.AddCommand(newMountCommand())
	cmd.AddCommand(newIPCommand())
	cmd.AddCommand(newCopyIDCommand())
	cmd.AddCommand(newConsoleCommand())

	// ── Observability ─────────────────────────────────────────────────────────
	cmd.AddCommand(newListCommand()) // now has --json
	cmd.AddCommand(newInspectCommand())
	cmd.AddCommand(newStatsCommand())
	cmd.AddCommand(newLogsCommand())
	cmd.AddCommand(newPortCommand())

	// ── SSH config ────────────────────────────────────────────────────────────
	cmd.AddCommand(newSSHConfigCommand())
	cmd.AddCommand(newAutostartCommand())

	// ── Images ────────────────────────────────────────────────────────────────
	cmd.AddCommand(newImagesCommand())
	cmd.AddCommand(newPullCommand())
	cmd.AddCommand(newImageUpdateCommand())
	cmd.AddCommand(newImageRmCommand())

	// ── Advanced VM management ────────────────────────────────────────────────
	cmd.AddCommand(newCloneCommand())
	cmd.AddCommand(newResizeCommand())
	cmd.AddCommand(newBalloonCommand())
	cmd.AddCommand(newSnapshotCommand())
	cmd.AddCommand(newPortForwardCommand())
	cmd.AddCommand(newExportCommand())
	cmd.AddCommand(newImportCommand())
	cmd.AddCommand(newSetKeyCommand())

	// ── Profiles ──────────────────────────────────────────────────────────────
	cmd.AddCommand(newProfileCommand())

	// ── Utilities ─────────────────────────────────────────────────────────────
	cmd.AddCommand(newMacAddressCommand())
	cmd.AddCommand(newCompletionCommand())
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newPruneCommand())

	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	defaultConfigPath := path.Join(homePath, ".spinup")

	var defaultQEMUExecutableName string
	switch runtime.GOARCH {
	case "arm64":
		defaultQEMUExecutableName = qemu.Aarch64ExecutableName
	case "amd64":
		defaultQEMUExecutableName = qemu.X8664ExecutableName
	default:
		return nil, ErrUnsupportedArchitecture
	}

	if v := os.Getenv("SPINUP_CONFIG_PATH"); v != "" {
		defaultConfigPath = v
	}
	if v := os.Getenv("SPINUP_QEMU_EXECUTABLE_NAME"); v != "" {
		defaultQEMUExecutableName = v
	}

	cmd.PersistentFlags().String("config-path", defaultConfigPath,
		"configuration path (env SPINUP_CONFIG_PATH)")
	cmd.PersistentFlags().String("qemu-executable-name", defaultQEMUExecutableName,
		"QEMU executable name (env SPINUP_QEMU_EXECUTABLE_NAME)")

	return cmd, nil
}

func newGlobalOptions(cmd *cobra.Command) (*globalOptions, error) {
	configPath, err := cmd.Flags().GetString("config-path")
	if err != nil {
		return nil, err
	}
	qemuExecutableName, err := cmd.Flags().GetString("qemu-executable-name")
	if err != nil {
		return nil, err
	}
	return &globalOptions{
		qemuExecutableName: qemuExecutableName,
		configPath:         configPath,
	}, nil
}

func newEngine(opts *globalOptions) (*engine.Engine, error) {
	return engine.New(&engine.NewOptions{
		QEMUExecutableName: opts.qemuExecutableName,
		Path:               opts.configPath,
		Writer:             os.Stderr,
	})
}
