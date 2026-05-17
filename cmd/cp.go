package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newCpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cp [vm:]/remote/path [vm:]/local/path",
		Short: "Copy files between the host and a running VM",
		Example: `  Download a file from vm1:
    spinup cp vm1:/etc/nginx/nginx.conf ./nginx.conf

  Upload a file to vm1:
    spinup cp ./nginx.conf vm1:/etc/nginx/nginx.conf`,
		Args: cobra.ExactArgs(2),
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

			src, dst := args[0], args[1]

			// Detect direction by looking for "vmname:/path" syntax.
			if vmName, remotePath, ok := splitVMPath(src); ok {
				// Download: vm -> host
				vm := mustFindVM(eng, vmName)
				if err := vm.CopyFrom(remotePath, dst); err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
			} else if vmName, remotePath, ok := splitVMPath(dst); ok {
				// Upload: host -> vm
				vm := mustFindVM(eng, vmName)
				if err := vm.CopyTo(src, remotePath); err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Println("Error: one argument must be in the form vm-name:/remote/path")
				os.Exit(1)
			}

			return nil
		},
	}
}

// splitVMPath splits "vmname:/some/path" into ("vmname", "/some/path", true).
// Returns ("", "", false) if the argument is not in VM path format.
func splitVMPath(s string) (vmName, remotePath string, ok bool) {
	// An absolute path like /etc/file should not be treated as vm:path.
	// We look for a colon that is not at position 0 and is preceded by
	// something that looks like a VM name (no slashes before the colon).
	before, after, found := strings.Cut(s, ":")
	if !found || strings.Contains(before, "/") || before == "" {
		return "", "", false
	}
	return before, after, true
}
