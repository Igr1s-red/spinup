package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newConsoleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "console <name>",
		Short: "Connect to the VM serial console (requires socat)",
		Args:  cobra.ExactArgs(1),
		Long: `Connect to the VM's serial console via a Unix socket.

Useful as an emergency escape hatch when SSH is unavailable (kernel panic,
misconfigured network, cloud-init failure, etc.).

Press Ctrl+] to disconnect from the console.

Requires socat:
  Linux:  sudo apt install socat
  macOS:  brew install socat`,
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
			vm := mustFindVM(eng, args[0])
			if err := vm.Console(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
