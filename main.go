package main

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
