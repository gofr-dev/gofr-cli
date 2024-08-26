package main

import (
	"gofr.dev/pkg/gofr"

	"github.com/gofr-dev/gofr-cli/bootstrap"
	"github.com/gofr-dev/gofr-cli/migration"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("init", bootstrap.Create)

	cli.SubCommand("version",
		func(*gofr.Context) (interface{}, error) {
			return CLIVersion, nil
		},
	)

	cli.SubCommand("migrate create", migration.Migrate)

	cli.Run()
}
