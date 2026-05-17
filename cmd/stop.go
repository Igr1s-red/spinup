package cmd

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Short: "Stop a running virtual machine",
		Use:   "stop [name]",
		Example: `  Stop vm1:
    spinup stop vm1

  Stop every running VM:
    spinup stop --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			all, _ := cmd.Flags().GetBool("all")

			if all && len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: cannot specify a VM name together with --all")
				os.Exit(1)
			}
			if !all && len(args) == 0 {
				fmt.Fprintln(os.Stderr, "Error: specify a VM name or use --all")
				os.Exit(1)
			}

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			if all {
				if err := stopAll(eng); err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				return nil
			}

			vm := mustFindVM(eng, args[0])
			if err := vm.Stop(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().Bool("all", false, "stop all running VMs")

	return cmd
}

const stopParallelism = 4

func stopAll(eng *engine.Engine) error {
	vms := eng.ListVirtualMachines()
	sort.Slice(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })

	var (
		mu     sync.Mutex
		failed []string
		wg     sync.WaitGroup
		sem    = make(chan struct{}, stopParallelism)
	)

	for _, vm := range vms {
		if s, err := vm.Status(); err != nil || s != engine.VirtualMachineStatusRunning {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(vm *engine.VirtualMachine) {
			defer wg.Done()
			defer func() { <-sem }()
			fmt.Fprintf(os.Stderr, "Stopping %q...\n", vm.Name)
			if err := vm.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Error stopping %q: %s\n", vm.Name, err)
				mu.Lock()
				failed = append(failed, vm.Name)
				mu.Unlock()
			}
		}(vm)
	}
	wg.Wait()

	if len(failed) > 0 {
		return fmt.Errorf("failed to stop: %v", failed)
	}
	return nil
}
