package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	cli := gofr.NewCMD()

	cli.SubCommand("version",
		func(*gofr.Context) (interface{}, error) {
			return CLIVersion, nil
		},
	)

	cli.Run()
}
