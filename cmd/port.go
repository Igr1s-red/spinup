package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newPortCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "port <name> [vm-port]",
		Short: "Print the host port mapped to a VM port",
		Args:  cobra.RangeArgs(1, 2),
		Example: `  Print all port forwards for vm1:
    spinup port vm1

  Print the host port mapped to VM port 80:
    spinup port vm1 80`,
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

			if len(args) == 2 {
				hostPort, err := resolveVMPort(vm, args[1])
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				fmt.Println(hostPort)
				return nil
			}

			printAllPorts(vm)
			return nil
		},
	}
}

// resolveVMPort returns the host port that maps to vmPort on the given VM.
func resolveVMPort(vm *engine.VirtualMachine, vmPort string) (string, error) {
	for _, nic := range vm.Config.Networks {
		if hp, ok := nic.PortForwards[vmPort]; ok {
			return hp, nil
		}
	}
	return "", fmt.Errorf("no port forward configured for VM port %s on %q", vmPort, vm.Name)
}

func printAllPorts(vm *engine.VirtualMachine) {
	type fwd struct{ vm, host string }
	var fwds []fwd
	for _, nic := range vm.Config.Networks {
		for vmPort, hostPort := range nic.PortForwards {
			fwds = append(fwds, fwd{vmPort, hostPort})
		}
	}
	sort.Slice(fwds, func(i, j int) bool { return fwds[i].vm < fwds[j].vm })
	for _, f := range fwds {
		fmt.Printf("%s -> 127.0.0.1:%s\n", f.vm, f.host)
	}
}
