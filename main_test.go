package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_Main_Version(t *testing.T) {
	old := os.Args
	os.Args = []string{"gofr-cli", "version"}

	t.Cleanup(func() {
		os.Args = old
	})

	// test util replaces the stdout with a buffer and returns us
	// any output that is printed on the stdout
	out := testutil.StdoutOutputForFunc(func() {
		main()
	})

	assert.Contains(t, out, "dev")
}

func Test_Main_init(t *testing.T) {

}
