package cmd

import (
	"fmt"
	"os"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "stats [name]",
		Short:   "Show resource stats for a VM (live I/O via QMP when running)",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup stats vm1",
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

			stats, err := vm.Stats()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			fmt.Print(engine.FormatStats(stats))
			return nil
		},
	}
}
