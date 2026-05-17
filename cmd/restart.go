package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRestartCommand() *cobra.Command {
	return &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Short: "Restart a running virtual machine",
		Use:   "restart [name]",
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
			if err := vm.Stop(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			if err := vm.Start(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
