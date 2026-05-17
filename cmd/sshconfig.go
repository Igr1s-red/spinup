package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSSHConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-config",
		Short: "Manage ~/.ssh/config entries for VMs",
		Long: `Write or remove SSH config entries in ~/.ssh/config.

Once written, standard tools (ssh, scp, rsync, VS Code Remote, Transmit)
connect to the VM by name without any spinup involvement:

  ssh vm1
  scp vm1:/etc/nginx/nginx.conf ./nginx.conf
  rsync -avz vm1:/var/www/ ./local-copy/`,
	}

	cmd.AddCommand(newSSHConfigWriteCommand())
	cmd.AddCommand(newSSHConfigRemoveCommand())

	return cmd
}

func newSSHConfigWriteCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "write [vm]",
		Short:   "Write (or update) an SSH config entry for a running VM",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup ssh-config write vm1",
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
			if err := vm.SSHConfigEntry(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ Added to ~/.ssh/config — you can now run: ssh %s\n", args[0])
			return nil
		},
	}
}

func newSSHConfigRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove [vm]",
		Aliases: []string{"rm"},
		Short:   "Remove the SSH config entry for a VM",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup ssh-config remove vm1",
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
			if err := vm.SSHConfigRemove(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
