package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/gofr-cli/bootstrap"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("init", bootstrap.Create)

	cli.SubCommand("version",
		func(*gofr.Context) (interface{}, error) {
			return CLIVersion, nil
		},
	)

	cli.Run()
}
