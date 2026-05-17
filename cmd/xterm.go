package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newXtermCommand() *cobra.Command {
	return &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Short: "Connect via SSH to a running virtual machine",
		Use:   "xterm [name]",
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
			if err := vm.SSHSessionWithXterm(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
