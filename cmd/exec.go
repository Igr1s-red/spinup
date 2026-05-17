package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newExecCommand() *cobra.Command {
	return &cobra.Command{
		Args: cobra.MinimumNArgs(2),
		Example: `  Execute "uname -a" in the virtual machine:
    spinup exec vm1 -- uname -a`,
		Short: "Execute a command in a running virtual machine",
		Use:   "exec [name] [command...]",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			if err := runExec(globalOptions, args[0], strings.Join(args[1:], " ")); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			return nil
		},
	}
}

func runExec(opts *globalOptions, name, command string) error {
	eng, err := newEngine(opts)
	if err != nil {
		return err
	}

	vm := eng.FindVirtualMachine(name)
	if vm == nil {
		return fmt.Errorf("virtual machine %q not found", name)
	}

	return vm.Exec(command)
}
