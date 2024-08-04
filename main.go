package main

import (
	"fmt"
	"os"

	"github.com/Igr1s-red/spinup/cmd"
)

func main() {
	cmd, err := cmd.New()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
