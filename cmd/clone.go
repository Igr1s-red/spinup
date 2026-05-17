package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCloneCommand() *cobra.Command {
	return &cobra.Command{
		Args:  cobra.ExactArgs(2),
		Use:   "clone [source] [destination]",
		Short: "Clone a stopped VM into a new linked copy-on-write VM",
		Example: `  spinup clone vm1 vm1-clone

The clone shares the source disk as a qcow2 backing file.
Only modified blocks are written to the new VM's disk.
The source VM must be stopped.`,
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

			clone, err := eng.CloneVirtualMachine(args[0], args[1])
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("Clone \"%s\" created successfully\n", clone.Name)
			return nil
		},
	}
}
