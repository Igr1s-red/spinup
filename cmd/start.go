package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

type startOptions struct {
	*globalOptions
	name        string
	all         bool
	cpu         int
	memory      int
	wait        bool
	waitTimeout time.Duration
	waitPort    string
}

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Short: "Start a stopped virtual machine",
		Use:   "start [name]",
		Example: `  Start vm1 with its stored config:
    spinup start vm1

  Start with more RAM for this run only (does not persist):
    spinup start vm1 -m 4096

  Start and block until SSH is ready:
    spinup start vm1 --wait

  Start every stopped VM:
    spinup start --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			cpu, _ := cmd.Flags().GetInt("cpu")
			memory, _ := cmd.Flags().GetInt("memory")
			wait, _ := cmd.Flags().GetBool("wait")
			waitTimeout, _ := cmd.Flags().GetDuration("wait-timeout")
			waitPort, _ := cmd.Flags().GetString("wait-port")
			all, _ := cmd.Flags().GetBool("all")

			if all && len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: cannot specify a VM name together with --all")
				os.Exit(1)
			}
			if !all && len(args) == 0 {
				fmt.Fprintln(os.Stderr, "Error: specify a VM name or use --all")
				os.Exit(1)
			}

			opts := &startOptions{
				globalOptions: globalOptions,
				all:           all,
				cpu:           cpu,
				memory:        memory,
				wait:          wait,
				waitTimeout:   waitTimeout,
				waitPort:      waitPort,
			}
			if len(args) > 0 {
				opts.name = args[0]
			}

			if err := runStart(opts); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().IntP("cpu", "c", 0, "override vCPU count for this run (0 = use stored config)")
	cmd.Flags().IntP("memory", "m", 0, "override RAM in MiB for this run (0 = use stored config)")
	cmd.Flags().Bool("wait", false, "block until SSH is accepting connections")
	cmd.Flags().Duration("wait-timeout", 2*time.Minute, "maximum time to wait for SSH or --wait-port")
	cmd.Flags().String("wait-port", "", "also wait for this VM port to accept connections (e.g. 80)")
	cmd.Flags().Bool("all", false, "start all stopped VMs (runs in parallel)")

	return cmd
}

func runStart(opts *startOptions) error {
	eng, err := newEngine(opts.globalOptions)
	if err != nil {
		return err
	}

	if opts.all {
		return startAll(eng, opts)
	}

	vm := eng.FindVirtualMachine(opts.name)
	if vm == nil {
		return fmt.Errorf("virtual machine %q not found", opts.name)
	}
	return startOne(vm, opts)
}

const startParallelism = 4

func startAll(eng *engine.Engine, opts *startOptions) error {
	vms := eng.ListVirtualMachines()
	sort.Slice(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })

	var (
		mu     sync.Mutex
		failed []string
		wg     sync.WaitGroup
		sem    = make(chan struct{}, startParallelism)
	)

	for _, vm := range vms {
		if s, err := vm.Status(); err != nil || s == engine.VirtualMachineStatusRunning {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(vm *engine.VirtualMachine) {
			defer wg.Done()
			defer func() { <-sem }()
			fmt.Fprintf(os.Stderr, "Starting %q...\n", vm.Name)
			if err := startOne(vm, opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting %q: %s\n", vm.Name, err)
				mu.Lock()
				failed = append(failed, vm.Name)
				mu.Unlock()
			}
		}(vm)
	}
	wg.Wait()

	if len(failed) > 0 {
		return fmt.Errorf("failed to start: %v", failed)
	}
	return nil
}

func startOne(vm *engine.VirtualMachine, opts *startOptions) error {
	if err := vm.StartWithOptions(engine.StartOptions{
		CPU:    opts.cpu,
		Memory: opts.memory,
	}); err != nil {
		return err
	}

	// Refresh SSH config so `ssh <name>` and rsync work immediately.
	if err := vm.SSHConfigEntry(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: SSH config entry not written: %s\n", err)
	}

	if opts.wait {
		ctx, cancel := context.WithTimeout(context.Background(), opts.waitTimeout)
		defer cancel()
		if err := vm.WaitForSSH(ctx); err != nil {
			return err
		}
	}

	if opts.waitPort != "" {
		hostPort, err := resolveVMPort(vm, opts.waitPort)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), opts.waitTimeout)
		defer cancel()
		return vm.WaitForPort(ctx, hostPort)
	}

	return nil
}
