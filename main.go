// main.go
package main

import (
	"os"

	"github.com/outport-app/outport/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
