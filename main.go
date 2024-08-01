package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/gofr-cli/project"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("init", project.Create)

	cli.SubCommand("version",
		func(*gofr.Context) (interface{}, error) {
			return CLIVersion, nil
		},
	)

	cli.Run()
}
