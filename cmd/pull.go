package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPullCommand() *cobra.Command {
	return &cobra.Command{
		Args:    cobra.ExactArgs(1),
		Short:   "Pull an image",
		Use:     "pull [name]",
		Example: "  spinup pull debian:bookworm",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			if err := runPull(globalOptions, args[0]); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			return nil
		},
	}
}

func runPull(opts *globalOptions, name string) error {
	eng, err := newEngine(opts)
	if err != nil {
		return err
	}

	img := eng.FindImage(name)
	if img == nil {
		return fmt.Errorf("image %q not found — run 'spinup images' to see available images", name)
	}

	return img.Pull()
}
