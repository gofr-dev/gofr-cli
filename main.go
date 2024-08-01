package main

import (
	"gofr.dev/gofr-cli/project"
	"gofr.dev/pkg/gofr"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("version", func(*gofr.Context) (interface{}, error) {
		return CLIVersion, nil
	})

	cli.SubCommand("init", project.Create)

	cli.Run()
}
