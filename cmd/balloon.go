package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newBalloonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "balloon [name] [mib]",
		Short: "Adjust the memory balloon target of a running VM",
		Long: `Sets the virtio-balloon target to the specified number of MiB.

The balloon driver can reclaim memory from the guest (shrink) or
return previously reclaimed memory (grow). The VM must be running.

The VM's configured memory (-m flag) remains unchanged — this only
adjusts the live balloon target. The change is not persisted; restart
the VM to restore the configured value.`,
		Args:    cobra.ExactArgs(2),
		Example: "  spinup balloon vm1 2048   # shrink to 2 GiB\n  spinup balloon vm1 4096   # restore to 4 GiB",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			mib, err := strconv.Atoi(args[1])
			if err != nil || mib < 64 {
				fmt.Printf("Error: target must be an integer >= 64 (MiB), got %q\n", args[1])
				os.Exit(1)
			}

			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			vm := mustFindVM(eng, args[0])

			if err := vm.Balloon(mib); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
