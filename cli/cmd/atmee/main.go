// Command atmee is the Atmee Avatar CLI.
package main

import (
	"os"

	"github.com/Atmanity/toolkit/cli/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}
