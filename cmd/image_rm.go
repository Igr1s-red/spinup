package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newImageRmCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "image-rm <image:version>",
		Short: "Remove a pulled image to reclaim disk space",
		Args:  cobra.ExactArgs(1),
		Example: `  Remove the cached Debian 12 image:
    spinup image-rm debian:bookworm`,
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
			img := eng.FindImage(args[0])
			if img == nil {
				fmt.Printf("Error: image %q not found\n", args[0])
				os.Exit(1)
			}
			if err := img.Remove(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
