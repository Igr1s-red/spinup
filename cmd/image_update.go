package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newImageUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update [image]",
		Short: "Re-pull a cached image to get the latest build",
		Long: `Downloads the latest version of an already-pulled image,
replacing the cached disk. Useful for Arch Linux (rolling)
or to pick up new Debian/Ubuntu nightly builds.

Existing VMs created from the old image are not affected.`,
		Args:    cobra.ExactArgs(1),
		Example: "  spinup image update arch:latest\n  spinup image update debian:bookworm",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			eng, err := newEngine(globalOptions)
			if err != nil {
				return err
			}

			img := eng.FindImage(args[0])
			if img == nil {
				fmt.Printf("Error: image %q not found — run 'spinup images' to list available images\n", args[0])
				os.Exit(1)
			}

			pulled, err := img.Pulled()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			if !pulled {
				fmt.Printf("Image %q has not been pulled yet — use 'spinup pull %s'\n", args[0], args[0])
				os.Exit(1)
			}

			if err := img.Update(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("Image %q updated to the latest build\n", args[0])
			return nil
		},
	}
}
