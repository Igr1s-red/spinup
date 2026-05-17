package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newPortForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward",
		Short: "Manage port forwards for a stopped VM",
	}
	cmd.AddCommand(
		newPortForwardAddCommand(),
		newPortForwardRemoveCommand(),
		newPortForwardListCommand(),
	)
	return cmd
}

// ── add ───────────────────────────────────────────────────────────────────────

func newPortForwardAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add [vm] [hostport-vmport]",
		Short: "Add a port forward to a stopped VM (takes effect on next start)",
		Args:  cobra.ExactArgs(2),
		Example: `  Forward host port 8080 to VM port 80:
    spinup port-forward add vm1 8080-80

  Forward host port 5432 to VM port 5432:
    spinup port-forward add vm1 5432-5432`,
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
			hostPort, vmPort, ok := strings.Cut(args[1], "-")
			if !ok {
				fmt.Println("Error: format must be hostport-vmport (e.g. 8080-80)")
				os.Exit(1)
			}
			// AddPortForward(hostPort, vmPort) — host port first, VM port second.
			if err := vm.AddPortForward(hostPort, vmPort); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Added port forward: host:%s -> vm:%s (restart VM to apply)\n", hostPort, vmPort)
			return nil
		},
	}
}

// ── remove ────────────────────────────────────────────────────────────────────

func newPortForwardRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove [vm] [vmport]",
		Aliases: []string{"rm"},
		Short:   "Remove a port forward from a stopped VM",
		Args:    cobra.ExactArgs(2),
		Example: "  spinup port-forward remove vm1 80",
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
			if err := vm.RemovePortForward(args[1]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Removed port forward for VM port %s (restart VM to apply)\n", args[1])
			return nil
		},
	}
}

// ── list ──────────────────────────────────────────────────────────────────────

func newPortForwardListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list [vm]",
		Aliases: []string{"ls"},
		Short:   "List port forwards for a VM",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup port-forward list vm1",
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

			type pfEntry struct{ vmPort, hostPort, nic string }
			var entries []pfEntry
			for i, n := range vm.Config.Networks {
				for vmPort, hostPort := range n.PortForwards {
					entries = append(entries, pfEntry{
						vmPort:   vmPort,
						hostPort: hostPort,
						nic:      fmt.Sprintf("nic%d (%s)", i, n.NetworkMode),
					})
				}
			}

			if len(entries) == 0 {
				fmt.Printf("No port forwards configured for %q\n", args[0])
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NIC\tVM PORT\tHOST PORT")
			for _, e := range entries {
				fmt.Fprintf(w, "%s\t%s\t%s\n", e.nic, e.vmPort, e.hostPort)
			}
			if err := w.Flush(); err != nil {
				return fmt.Errorf("flush table: %w", err)
			}
			return nil
		},
	}
}
