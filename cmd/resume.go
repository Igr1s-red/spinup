package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newResumeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <name>",
		Short: "Resume a paused VM",
		Args:  cobra.ExactArgs(1),
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
			if err := vm.Resume(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
