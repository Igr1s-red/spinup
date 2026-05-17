package cmd

import (
	"fmt"
	"os"

	"github.com/Igr1s-red/spinup/engine"
)

func mustFindVM(eng *engine.Engine, name string) *engine.VirtualMachine {
	vm := eng.FindVirtualMachine(name)
	if vm == nil {
		fmt.Printf("Error: virtual machine %q not found\n", name)
		os.Exit(1)
	}
	return vm
}
