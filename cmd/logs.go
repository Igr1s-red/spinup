package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

type logsOptions struct {
	name          string
	follow        bool
	globalOptions *globalOptions
}

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Short: "Fetch the QEMU console logs of a virtual machine",
		Use:   "logs [name]",
		Example: `  Print all logs for vm1:
    spinup logs vm1

  Follow (tail) logs in real time:
    spinup logs vm1 -f`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			follow, err := cmd.Flags().GetBool("follow")
			if err != nil {
				return err
			}

			opts := &logsOptions{
				name:          args[0],
				follow:        follow,
				globalOptions: globalOptions,
			}

			if err := runLogs(opts); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolP("follow", "f", false, "follow log output (like tail -F, handles restarts)")

	return cmd
}

func runLogs(opts *logsOptions) error {
	eng, err := newEngine(opts.globalOptions)
	if err != nil {
		return err
	}

	vm := eng.FindVirtualMachine(opts.name)
	if vm == nil {
		return fmt.Errorf("virtual machine %q not found", opts.name)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return vm.Logs(ctx, opts.follow)
}
