package cmd

import (
	"fmt"
	"os"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file> <name>",
		Short: "Import a qcow2 disk as a new VM",
		Args:  cobra.ExactArgs(2),
		Example: `  Import a previously exported VM:
    spinup import vm1.qcow2 restored

  Specify image key to match SSH user:
    spinup import ubuntu.qcow2 myvm --image ubuntu:jammy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			image, _ := cmd.Flags().GetString("image")
			cpu, _ := cmd.Flags().GetInt("cpu")
			memory, _ := cmd.Flags().GetInt("memory")

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			vm, err := eng.ImportVirtualMachine(engine.ImportOptions{
				DiskPath: args[0],
				Name:     args[1],
				Image:    image,
				CPU:      cpu,
				Memory:   memory,
			})
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("VM %q imported. Start with: spinup start %s\n", vm.Name, vm.Name)
			return nil
		},
	}

	cmd.Flags().String("image", "debian:bookworm",
		"image key that matches the guest OS (sets the SSH user; e.g. ubuntu:jammy)")
	cmd.Flags().IntP("cpu", "c", 2, "number of vCPUs")
	cmd.Flags().IntP("memory", "m", 1024, "RAM in MiB")

	return cmd
}
