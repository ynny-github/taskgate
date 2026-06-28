// taskgate/main.go
package main

import (
	"os"

	"github.com/ynny-github/taskgate/taskgate/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
