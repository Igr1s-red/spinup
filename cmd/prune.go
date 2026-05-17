package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove all stopped VMs (dry run without --force)",
		Long: `List all stopped VMs and, with --force, remove them and free their disk space.

Without --force this is a safe dry run that only prints what would be removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			force, _ := cmd.Flags().GetBool("force")

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			vms := eng.ListVirtualMachines()
			sort.Slice(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })

			var stopped []*engine.VirtualMachine
			for _, vm := range vms {
				if s, err := vm.Status(); err == nil && s == engine.VirtualMachineStatusStopped {
					stopped = append(stopped, vm)
				}
			}

			if len(stopped) == 0 {
				fmt.Println("No stopped VMs to remove.")
				return nil
			}

			fmt.Printf("Stopped VMs (%d):\n", len(stopped))
			for _, vm := range stopped {
				fmt.Printf("  %s\n", vm.Name)
			}

			if !force {
				fmt.Println("\nRe-run with --force to remove them.")
				return nil
			}

			fmt.Println()
			var failed []string
			for _, vm := range stopped {
				fmt.Fprintf(os.Stderr, "Removing %q...\n", vm.Name)
				if err := vm.Remove(); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing %q: %s\n", vm.Name, err)
					failed = append(failed, vm.Name)
				}
			}

			if len(failed) > 0 {
				fmt.Printf("Error: failed to remove: %v\n", failed)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().Bool("force", false, "remove the stopped VMs (default: dry run)")
	return cmd
}
