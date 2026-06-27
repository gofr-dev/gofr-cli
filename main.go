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

	cli.SubCommand("store init", store.InitStore,
		gofr.AddDescription("Initialize a new data store"),
		gofr.AddHelp(`Initialize a new data store

Usage:
  gofr store init -name=<store_name>

Arguments:
  -name=<store_name>    Name of the store package to create (required)`),
	)

	cli.SubCommand("store generate", store.GenerateStore,
		gofr.AddDescription("Generate store methods"),
		gofr.AddHelp(`Generate store methods from configuration

Usage:
  gofr store generate [options]

Arguments:
  -name=<store_name>      Name of the store package (optional)
  -config=<config_path>   Path to the YAML config file (optional)`),
	)

	cli.SubCommand("init", bootstrap.Create,
		gofr.AddDescription("Initialize a new GoFr project"),
		gofr.AddHelp(`Initialize a new GoFr project with basic structure

Usage:
  gofr init -name=<project_name> [options]

Arguments:
  -name=<project_name>    Module name for the new project (required)
  -gofr=<version>         GoFr framework version (default: 1.17.0) (optional)`),
	)

	cli.SubCommand("version",
		func(*gofr.Context) (any, error) {
			return CLIVersion, nil
		},
		gofr.AddDescription("Display CLI version"),
		gofr.AddHelp("Show the current version of the GoFr CLI"),
	)

	cli.SubCommand("migrate create", migration.Migrate,
		gofr.AddDescription("Create a new database migration"),
		gofr.AddHelp(`Create a new database migration file

Usage:
  gofr migrate create -name=<migration_name>

Arguments:
  -name=<migration_name>  Migration name in snake_case or kebab-case (required)
                          Will be converted to camelCase with timestamp prefix`),
	)

	cli.SubCommand("wrap grpc server", wrap.BuildGRPCGoFrServer,
		gofr.AddDescription("Generate gRPC server wrapper"),
		gofr.AddHelp(`Generate gRPC server wrapper with GoFr integration

Usage:
  gofr wrap grpc server -proto=<proto_file>

Arguments:
  -proto=<proto_file>     Path to the .proto file defining your gRPC service (required)`),
	)

	cli.SubCommand("wrap grpc client", wrap.BuildGRPCGoFrClient,
		gofr.AddDescription("Generate gRPC client wrapper"),
		gofr.AddHelp(`Generate gRPC client wrapper with GoFr integration

Usage:
  gofr wrap grpc client -proto=<proto_file>

Arguments:
  -proto=<proto_file>     Path to the .proto file defining your gRPC service (required)`),
	)

	cli.Run()
}
