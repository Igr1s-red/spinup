package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRenameCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a virtual machine (VM must be stopped)",
		Args:  cobra.ExactArgs(2),
		Example: `  Rename vm1 to dev-box:
    spinup rename vm1 dev-box`,
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
			if _, err := eng.RenameVirtualMachine(args[0], args[1]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
