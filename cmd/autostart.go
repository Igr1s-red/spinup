package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAutostartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autostart",
		Short: "Configure a VM to start automatically at login",
		Long: `Manages automatic VM startup at login.

macOS: writes a launchd plist to ~/Library/LaunchAgents/ and loads it immediately.
Linux: writes a systemd user unit to ~/.config/systemd/user/ and enables it.

The VM will be started automatically when you log in, without any manual
spinup invocation.`,
	}

	cmd.AddCommand(newAutostartEnableCommand())
	cmd.AddCommand(newAutostartDisableCommand())
	cmd.AddCommand(newAutostartStatusCommand())

	return cmd
}

func newAutostartEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "enable [vm]",
		Short:   "Enable autostart for a VM",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup autostart enable vm1",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			vm := mustFindVM(eng, args[0])
			if err := vm.AutostartEnable(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}

func newAutostartDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "disable [vm]",
		Short:   "Disable autostart for a VM",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup autostart disable vm1",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			vm := mustFindVM(eng, args[0])
			if err := vm.AutostartDisable(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}

func newAutostartStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "status [vm]",
		Short:   "Show whether autostart is enabled for a VM",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup autostart status vm1",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			vm := mustFindVM(eng, args[0])
			enabled, err := vm.AutostartEnabled()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			if enabled {
				fmt.Printf("Autostart: enabled\n")
			} else {
				fmt.Printf("Autostart: disabled\n")
			}
			return nil
		},
	}
}
