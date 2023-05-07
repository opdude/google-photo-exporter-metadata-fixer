package main

import (
	"fmt"

	"github.com/go-errors/errors"
	cmd "github.com/opdude/google-photo-exporter-metadata-fixer/cmd/google-photo-exporter-organizer"
)

// main is the entrypoint for the application.
func main() {
	err := cmd.Execute()
	if err != nil {
		fmt.Println(err.(*errors.Error).ErrorStack())
	}
}
