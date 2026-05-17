package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newWaitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait <name>",
		Short: "Block until a VM reaches a given state",
		Args:  cobra.ExactArgs(1),
		Example: `  Wait for vm1 to be running (e.g. after spinup start):
    spinup wait vm1 --running

  Wait for vm1 to stop (scripting):
    spinup wait vm1 --stopped

  Wait for VM port 80 to accept connections:
    spinup wait vm1 --wait-port 80`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			running, _ := cmd.Flags().GetBool("running")
			stopped, _ := cmd.Flags().GetBool("stopped")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			waitPort, _ := cmd.Flags().GetString("wait-port")

			if running && stopped {
				fmt.Fprintln(os.Stderr, "Error: --running and --stopped are mutually exclusive")
				os.Exit(1)
			}
			if !running && !stopped && waitPort == "" {
				fmt.Fprintln(os.Stderr, "Error: specify --running, --stopped, or --wait-port")
				os.Exit(1)
			}

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			vm := mustFindVM(eng, args[0])

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			if running {
				if err := vm.WaitForStatus(ctx, engine.VirtualMachineStatusRunning); err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				return nil
			}

			if stopped {
				if err := vm.WaitForStatus(ctx, engine.VirtualMachineStatusStopped); err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				return nil
			}

			// --wait-port: resolve VM port → host port, then wait.
			hostPort, err := resolveVMPort(vm, waitPort)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			if err := vm.WaitForPort(ctx, hostPort); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().Bool("running", false, "wait until the VM is running")
	cmd.Flags().Bool("stopped", false, "wait until the VM is stopped")
	cmd.Flags().Duration("timeout", 5*time.Minute, "maximum wait time")
	cmd.Flags().String("wait-port", "", "wait until this VM port (e.g. 80) is accepting connections")

	return cmd
}
