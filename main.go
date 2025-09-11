package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/cli/gofr/bootstrap"
	"gofr.dev/cli/gofr/migration"
	"gofr.dev/cli/gofr/wrap"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("init", bootstrap.Create)

	cli.SubCommand("version",
		func(*gofr.Context) (any, error) {
			return CLIVersion, nil
		},
	)

	cli.SubCommand("migrate create", migration.Migrate)

	cli.SubCommand("wrap grpc server", wrap.BuildGRPCGoFrServer)

	cli.SubCommand("wrap grpc client", wrap.BuildGRPCGoFrClient)

	cli.Run()
}
