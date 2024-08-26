package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func Test_Main_Version(t *testing.T) {
	setArgs(t, "version")

	// test util replaces the stdout with a buffer and returns us
	// any output that is printed on the stdout
	out := testutil.StdoutOutputForFunc(func() {
		main()
	})

	assert.Contains(t, out, "dev")
}

func setArgs(t *testing.T, args ...string) {
	t.Helper()

	oldArgs := os.Args

	os.Args = append([]string{"gofr-cli"}, args...)

	t.Cleanup(func() {
		os.Args = oldArgs
	})
}
