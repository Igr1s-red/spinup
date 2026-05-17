package cmd

import (
	"fmt"
	"os"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newMountCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mount",
		Short: "Mount or unmount VM directories via sshfs",
		Long: `Mount remote VM directories on the host filesystem using sshfs.

Requires sshfs to be installed:
  macOS: brew install macfuse && brew install sshfs
  Linux: sudo apt install sshfs  (or pacman -S sshfs)`,
	}

	cmd.AddCommand(newMountMountCommand())
	cmd.AddCommand(newMountUnmountCommand())

	return cmd
}

func newMountMountCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add [vm] [remote-path] [local-dir]",
		Short: "Mount a VM directory locally via sshfs",
		Args:  cobra.ExactArgs(3),
		Example: `  Mount /home/debian from vm1 to ./vm1-home:
    spinup mount add vm1 /home/debian ./vm1-home

  Mount /var/www:
    spinup mount add vm1 /var/www ./www`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}
			vm := mustFindVM(eng, args[0])
			if err := vm.Mount(args[1], args[2]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}

func newMountUnmountCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove [local-dir]",
		Aliases: []string{"rm", "umount"},
		Short:   "Unmount a previously mounted sshfs directory",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup mount remove ./vm1-home",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}
			// Unmount is not VM-specific; we just need an engine for consistency.
			// Create a dummy VM for the call.
			_ = eng
			if err := engine.UnmountDir(args[0]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
