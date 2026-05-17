package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSetKeyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-key <name>",
		Short: "Rotate the VM's SSH key pair (VM must be stopped)",
		Args:  cobra.ExactArgs(1),
		Long: `Generate a new Ed25519 SSH key pair for the VM and rebuild its cloud-init ISO.

The VM must be stopped. On next boot cloud-init re-runs (because the instance-id
in the ISO changes) and installs the new authorized key. The old key is replaced
on disk immediately.`,
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
			if err := vm.SetKey(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
