package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCopyIDCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy-id [name]",
		Short: "Install your SSH public key into a running VM",
		Long: `Copies your local SSH public key into the VM's ~/.ssh/authorized_keys.
After this, standard SSH clients connect to the VM using your own key
without needing spinup's managed key.

Auto-discovers ~/.ssh/id_ed25519.pub, id_rsa.pub, or id_ecdsa.pub.
Use --key to specify a different key file.`,
		Args: cobra.ExactArgs(1),
		Example: `  Install your default key:
    spinup copy-id vm1

  Install a specific key:
    spinup copy-id vm1 --key ~/.ssh/work_ed25519.pub

  After this, you can use standard tools directly:
    ssh vm1-user@$(spinup ip vm1)
    ansible -i $(spinup ip vm1), all -m ping`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			keyPath, _ := cmd.Flags().GetString("key")

			eng, err := newEngine(globalOptions)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}

			vm := mustFindVM(eng, args[0])

			if err := vm.CopyID(keyPath); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().String("key", "", "path to the public key file to install (default: auto-detect ~/.ssh/id_*.pub)")
	return cmd
}
