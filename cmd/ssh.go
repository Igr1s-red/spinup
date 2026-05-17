package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSSHCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [name]",
		Short: "Open an interactive SSH session to a running VM",
		Long: `Opens an interactive SSH shell inside a running VM.

Use --details to print connection info for use in other SSH clients
instead of opening an interactive session.`,
		Args: cobra.ExactArgs(1),
		Example: `  Connect interactively:
    spinup ssh vm1

  Print connection string for scripting:
    spinup ssh vm1 --details

  One-liner shortcut (macOS/Linux):
    $(spinup ssh vm1 --command)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			details, _ := cmd.Flags().GetBool("details")
			command, _ := cmd.Flags().GetBool("command")

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			vm := mustFindVM(eng, args[0])

			// --command: print a raw ssh invocation for use in $(...) or scripts
			if command {
				info, err := vm.SSHConnectionDetails()
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				fmt.Printf("ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"+
					" -o IdentitiesOnly=yes -p %d -i %s %s@%s\n",
					info.Port, info.PrivateKey, info.Username, info.Host)
				return nil
			}

			// --details: print human-readable connection info
			if details {
				info, err := vm.SSHConnectionDetails()
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				fmt.Printf("Host:        %s\n", info.Host)
				fmt.Printf("Port:        %d\n", info.Port)
				fmt.Printf("User:        %s\n", info.Username)
				fmt.Printf("Private key: %s\n", info.PrivateKey)
				fmt.Printf("\nSSH command:\n  ssh -o StrictHostKeyChecking=no"+
					" -o UserKnownHostsFile=/dev/null -o IdentitiesOnly=yes"+
					" -p %d -i %s %s@%s\n",
					info.Port, info.PrivateKey, info.Username, info.Host)
				return nil
			}

			// Default: open interactive shell
			if err := vm.SSHSessionWithXterm(); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().Bool("details", false, "print connection details instead of opening a shell")
	cmd.Flags().Bool("command", false, "print the raw ssh command (for use in scripts)")

	return cmd
}
