package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "export <name> [file]",
		Short: "Export a VM's disk as a standalone compressed qcow2",
		Args:  cobra.RangeArgs(1, 2),
		Example: `  Export vm1 to vm1.qcow2 in the current directory:
    spinup export vm1

  Export to a specific path:
    spinup export vm1 /backups/vm1-$(date +%F).qcow2`,
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
			outPath := ""
			if len(args) == 2 {
				outPath = args[1]
			}
			if err := vm.Export(outPath); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
