package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newResizeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resize [name] [size-gb]",
		Short: "Grow a stopped VM's disk to a new size in GB",
		Long: `Grows the VM's disk image to the specified size.
The VM must be stopped. Shrinking is not supported.

After starting the VM, expand the filesystem inside the guest:
  Debian/Ubuntu:  sudo growpart /dev/vda 1 && sudo resize2fs /dev/vda1
  Fedora/Arch:    sudo growpart /dev/vda 1 && sudo xfs_growfs /
  (partition layout may vary — adjust /dev/vda1 to match your setup)`,
		Args:    cobra.ExactArgs(2),
		Example: "  spinup resize vm1 40",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			sizeGB, err := strconv.Atoi(args[1])
			if err != nil || sizeGB < 1 {
				fmt.Printf("Error: size must be a positive integer (GB), got %q\n", args[1])
				os.Exit(1)
			}

			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			vm := mustFindVM(eng, args[0])

			if err := vm.Resize(sizeGB); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
