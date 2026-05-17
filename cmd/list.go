package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Short:   "List virtual machines",
		Use:     "list",
		Aliases: []string{"ls"},
		Example: `  spinup list
  spinup list --json
  spinup list --json | jq '.[] | select(.status == "running") | .name'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")

			if err := runList(globalOptions, asJSON); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}

// listVM is the JSON-serialisable view of a VM for list output.
type listVM struct {
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	CPU          int      `json:"cpu"`
	MemoryMiB    int      `json:"memory_mib"`
	DiskSizeGB   int      `json:"disk_size_gb"`
	Status       string   `json:"status"`
	PortForwards []string `json:"port_forwards"`
}

func runList(opts *globalOptions, asJSON bool) error {
	eng, err := newEngine(opts)
	if err != nil {
		return err
	}

	vms := eng.ListVirtualMachines()

	entries := make([]listVM, 0, len(vms))
	for _, vm := range vms {
		status, err := vm.Status()
		if err != nil {
			return err
		}

		var forwards []string
		for _, n := range vm.Config.Networks {
			for vmPort, hostPort := range n.PortForwards {
				forwards = append(forwards, fmt.Sprintf("%s->%s", hostPort, vmPort))
			}
		}

		entries = append(entries, listVM{
			Name:         vm.Name,
			Image:        vm.Config.Image,
			CPU:          vm.Config.CPU,
			MemoryMiB:    vm.Config.Memory,
			DiskSizeGB:   vm.Config.DiskSize,
			Status:       string(status),
			PortForwards: forwards,
		})
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	// Table output.
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		statusStr := e.Status
		if len(statusStr) > 0 {
			statusStr = strings.ToUpper(statusStr[:1]) + statusStr[1:]
		}
		rows = append(rows, []string{
			e.Name,
			e.Image,
			fmt.Sprintf("%d CPU  %d MiB  %d GB", e.CPU, e.MemoryMiB, e.DiskSizeGB),
			strings.Join(e.PortForwards, ", "),
			statusStr,
		})
	}

	writeTable(&writeTableOptions{
		writer: os.Stdout,
		header: []string{"Name", "Image", "Resources", "Port Forwards", "Status"},
		rows:   rows,
	})
	return nil
}
