package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/steveclarke/outport/cmd"
	"github.com/steveclarke/outport/internal/ui"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if !errors.Is(err, cmd.ErrSilent) {
			fmt.Fprintln(os.Stderr, err)
			if hint := cmd.ErrorHint(err); hint != "" {
				fmt.Fprintln(os.Stderr, ui.DimStyle.Render("Hint: "+hint))
			}
		}
		os.Exit(1)
	}
}
