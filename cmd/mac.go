package cmd

import (
	"fmt"
	"os"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newMacAddressCommand() *cobra.Command {
	return &cobra.Command{
		Short:   "Generate a random locally administered unicast MAC address",
		Use:     "mac",
		Example: "  spinup mac",
		RunE: func(cmd *cobra.Command, args []string) error {
			mac, err := engine.RandomMAC()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Println(mac)
			return nil
		},
	}
}
