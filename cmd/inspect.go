package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "inspect [name]",
		Short:   "Display full configuration and runtime details of a VM as JSON",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup inspect vm1\n  spinup inspect vm1 | jq .config.CPU",
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
			info, err := vm.Inspect()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		},
	}
}
