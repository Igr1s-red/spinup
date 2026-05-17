package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newImagesCommand() *cobra.Command {
	return &cobra.Command{
		Short: "List available images",
		Use:   "images",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			if err := runImages(globalOptions); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			return nil
		},
	}
}

func runImages(opts *globalOptions) error {
	eng, err := newEngine(opts)
	if err != nil {
		return err
	}

	imgs := eng.ListImages()

	tableRows := make([][]string, 0, len(imgs))
	for _, image := range imgs {
		pulled := "No"
		if ok, err := image.Pulled(); err != nil {
			return fmt.Errorf("check image %s:%s: %w", image.Name, image.Version, err)
		} else if ok {
			pulled = "Yes"
		}

		dynamic := "-"
		if image.Dynamic {
			dynamic = "✓"
		}

		tableRows = append(tableRows, []string{
			fmt.Sprintf("%s:%s", image.Name, image.Version),
			image.Description,
			dynamic,
			pulled,
		})
	}

	writeTable(&writeTableOptions{
		writer: os.Stdout,
		header: []string{"Image", "Description", "Dynamic", "Pulled"},
		rows:   tableRows,
	})

	return nil
}
