package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newProfileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named VM creation profiles",
		Long: `Profiles store named sets of VM defaults.
Create a profile once, then use it with 'spinup run --profile <name>'.
Explicit flags always override profile values.`,
	}

	cmd.AddCommand(newProfileSaveCommand())
	cmd.AddCommand(newProfileListCommand())
	cmd.AddCommand(newProfileShowCommand())
	cmd.AddCommand(newProfileDeleteCommand())

	return cmd
}

// ── save ──────────────────────────────────────────────────────────────────────

func newProfileSaveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save [name]",
		Short: "Save a named profile with the given defaults",
		Args:  cobra.ExactArgs(1),
		Example: `  Save a dev profile:
    spinup profile save dev -i debian:bookworm -c 4 -m 4096 -d 20

  Save a minimal profile:
    spinup profile save tiny -i debian:bookworm -c 1 -m 512

  Use a profile:
    spinup run myvm --profile dev`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			image, _ := cmd.Flags().GetString("image")
			cpu, _ := cmd.Flags().GetInt("cpu")
			memory, _ := cmd.Flags().GetInt("memory")
			diskSize, _ := cmd.Flags().GetInt("disk-size")
			networks, _ := cmd.Flags().GetStringArray("network")
			shares, _ := cmd.Flags().GetStringArray("share")
			userData, _ := cmd.Flags().GetString("user-data")

			var nets []engine.NetworkInterfaceConfig
			for _, n := range networks {
				nets = append(nets, parseNetworkFlag(n))
			}

			var folders []engine.SharedFolder
			for _, s := range shares {
				host, tag, ok := strings.Cut(s, ":")
				if !ok {
					fmt.Fprintf(os.Stderr, "warning: skipping malformed --share %q\n", s)
					continue
				}
				folders = append(folders, engine.SharedFolder{HostPath: host, Tag: tag})
			}

			var udContent string
			if userData != "" {
				b, err := os.ReadFile(userData)
				if err != nil {
					fmt.Printf("Error: reading user-data: %s\n", err)
					os.Exit(1)
				}
				udContent = string(b)
			}

			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			p := &engine.Profile{
				Name:          args[0],
				Image:         image,
				CPU:           cpu,
				Memory:        memory,
				DiskSize:      diskSize,
				Networks:      nets,
				SharedFolders: folders,
				UserData:      udContent,
			}

			if err := eng.SaveProfile(p); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("Profile %q saved\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringP("image", "i", "", "default image")
	cmd.Flags().IntP("cpu", "c", 0, "default vCPU count (0 = no default)")
	cmd.Flags().IntP("memory", "m", 0, "default RAM in MiB (0 = no default)")
	cmd.Flags().IntP("disk-size", "d", 0, "default disk size in GB (0 = no default)")
	cmd.Flags().StringArray("network", nil, "default network config (same format as spinup run)")
	cmd.Flags().StringArray("share", nil, "default shared folders")
	cmd.Flags().String("user-data", "", "path to default cloud-init user-data file")

	return cmd
}

// ── list ──────────────────────────────────────────────────────────────────────

func newProfileListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			profiles, err := eng.ListProfiles()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			if len(profiles) == 0 {
				fmt.Println("No profiles saved. Use 'spinup profile save' to create one.")
				return nil
			}

			rows := make([][]string, 0, len(profiles))
			for _, p := range profiles {
				image := p.Image
				if image == "" {
					image = "(any)"
				}
				res := fmt.Sprintf("%d CPU  %d MiB  %d GB",
					p.CPU, p.Memory, p.DiskSize)
				if p.CPU == 0 && p.Memory == 0 && p.DiskSize == 0 {
					res = "(defaults)"
				}
				rows = append(rows, []string{p.Name, image, res})
			}

			writeTable(&writeTableOptions{
				writer: os.Stdout,
				header: []string{"Profile", "Image", "Resources"},
				rows:   rows,
			})
			return nil
		},
	}
}

// ── show ──────────────────────────────────────────────────────────────────────

func newProfileShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "show [name]",
		Short:   "Show full details of a profile as JSON",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup profile show dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			p, err := eng.LoadProfile(args[0])
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(p)
		},
	}
}

// ── delete ────────────────────────────────────────────────────────────────────

func newProfileDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "delete [name]",
		Aliases: []string{"rm"},
		Short:   "Delete a saved profile",
		Args:    cobra.ExactArgs(1),
		Example: "  spinup profile delete dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}
			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			if err := eng.DeleteProfile(args[0]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("Profile %q deleted\n", args[0])
			return nil
		},
	}
}
