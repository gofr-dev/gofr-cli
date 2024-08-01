package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/gofr-cli/version"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("version", func(_ *gofr.Context) (interface{}, error) {
		return version.CLIVersion, nil
	})

	cli.Run()
}
