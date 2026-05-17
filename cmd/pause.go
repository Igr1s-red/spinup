package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPauseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <name>",
		Short: "Suspend a running VM's CPU (stays in memory)",
		Args:  cobra.ExactArgs(1),
		Example: `  Pause vm1:
    spinup pause vm1

  Resume later:
    spinup resume vm1`,
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
			if err := vm.Pause(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
