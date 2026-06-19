// taskgate/main_test.go
package main_test

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/ynny-github/taskgate/taskgate/cmd"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"taskgate": func() int {
			return cmd.Run(os.Args[1:], os.Stdout, os.Stderr)
		},
	}))
}

func TestShow(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/show",
	})
}
