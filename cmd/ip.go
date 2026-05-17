package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newIPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip [name]",
		Short: "Print the primary IP address of a running VM",
		Long: `Connects via SSH and runs hostname -I to retrieve the VM's IP address.
Useful for bridged and host-only network modes where the VM has its own LAN IP.`,
		Args:    cobra.ExactArgs(1),
		Example: "  spinup ip vm1\n  curl http://$(spinup ip vm1)/api/health",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			all, _ := cmd.Flags().GetBool("all")

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			vm := mustFindVM(eng, args[0])

			if all {
				ips, err := vm.AllIPs()
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
				fmt.Println(strings.Join(ips, "\n"))
				return nil
			}

			ip, err := vm.IP()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Println(ip)
			return nil
		},
	}

	cmd.Flags().Bool("all", false, "print all IP addresses, one per line")
	return cmd
}
