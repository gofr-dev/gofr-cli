package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/cli/gofr/bootstrap"
	"gofr.dev/cli/gofr/migration"
	"gofr.dev/cli/gofr/store"
	"gofr.dev/cli/gofr/wrap"
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

	cli.SubCommand("wrap grpc server", wrap.BuildGRPCGoFrServer)

	cli.SubCommand("wrap grpc client", wrap.BuildGRPCGoFrClient)

	cli.SubCommand("store init", store.InitStore)

	cli.SubCommand("store generate", store.GenerateStore)

	cli.Run()
}
