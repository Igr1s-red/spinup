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
	// driver             string
	qemuExecutableName string
	configPath         string
}

func New() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Short: "Spin up Linux VMs with QEMU",
		Use:   "spinup",
	}

	// cmd.AddCommand(newLogsCommand())
	cmd.AddCommand(newCompletionCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newExecCommand())
	cmd.AddCommand(newImagesCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newPullCommand())
	cmd.AddCommand(newRemoveCommand())
	cmd.AddCommand(newRestartCommand())
	cmd.AddCommand(newSSHCommand())
	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newStopCommand())
	// cmd.AddCommand(newXtermCommand())
	cmd.AddCommand(newMacAddressCommand())

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

	configPathEnv := os.Getenv("spinup_CONFIG_PATH")
	if configPathEnv != "" {
		defaultConfigPath = configPathEnv
	}

	qemuExecutableNameEnv := os.Getenv("spinup_QEMU_EXECUTABLE_NAME")
	if qemuExecutableNameEnv != "" {
		defaultQEMUExecutableName = qemuExecutableNameEnv
	}

	cmd.PersistentFlags().String("config-path", defaultConfigPath, "configuration path (env spinup_CONFIG_PATH)")
	cmd.PersistentFlags().String("qemu-executable-name", defaultQEMUExecutableName, "qemu executable name (env spinup_QEMU_EXECUTABLE_NAME)")

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
