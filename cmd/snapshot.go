package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newSnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Short: "Manage virtual machine snapshots",
		Use:   "snapshot",
	}
	cmd.AddCommand(newSnapshotCreateCommand())
	cmd.AddCommand(newSnapshotListCommand())
	cmd.AddCommand(newSnapshotRestoreCommand())
	cmd.AddCommand(newSnapshotDeleteCommand())
	return cmd
}

func newSnapshotCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create [vm] [snapshot-name]",
		Short: "Create a snapshot of a stopped VM (name is optional — defaults to timestamp)",
		Args:  cobra.RangeArgs(1, 2),
		Example: `  Named snapshot:
    spinup snapshot create vm1 before-upgrade

  Auto-timestamp name:
    spinup snapshot create vm1`,
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
			name := ""
			if len(args) > 1 {
				name = args[1]
			}
			if err := vm.CreateSnapshot(name); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			if name == "" {
				name = "snapshot-" + time.Now().Format("2006-01-02T150405")
				fmt.Printf("Snapshot created (auto-named: %s)\n", name)
			} else {
				fmt.Printf("Snapshot %q created\n", name)
			}
			return nil
		},
	}
}

func newSnapshotListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   "list [vm]",
		Short: "List snapshots of a virtual machine",
		Example: `  spinup snapshot list vm1
  spinup snapshot list vm1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")

			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}
			vm := mustFindVM(eng, args[0])
			snaps, err := vm.ListSnapshots()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			if asJSON {
				b, err := json.MarshalIndent(snaps, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal: %w", err)
				}
				fmt.Println(string(b))
				return nil
			}

			if len(snaps) == 0 {
				fmt.Println("No snapshots found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tVM STATE\tCREATED")
			for _, s := range snaps {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					s.ID, s.Name,
					formatSnapSize(s.VMSize),
					s.CreatedAt.Format(time.DateTime),
				)
			}
			if err := w.Flush(); err != nil {
				return fmt.Errorf("flush: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}

func newSnapshotRestoreCommand() *cobra.Command {
	return &cobra.Command{
		Args:    cobra.ExactArgs(2),
		Use:     "restore [vm] [snapshot-name]",
		Short:   "Restore a VM to a named snapshot (VM must be stopped)",
		Example: "  spinup snapshot restore vm1 before-upgrade",
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
			if err := vm.RestoreSnapshot(args[1]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Restored %q to snapshot %q\n", args[0], args[1])
			return nil
		},
	}
}

func newSnapshotDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Args:    cobra.ExactArgs(2),
		Use:     "delete [vm] [snapshot-name]",
		Short:   "Delete a named snapshot (VM must be stopped)",
		Example: "  spinup snapshot delete vm1 before-upgrade",
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
			if err := vm.DeleteSnapshot(args[1]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Snapshot %q deleted\n", args[1])
			return nil
		},
	}
}

func formatSnapSize(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
