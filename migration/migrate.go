package migration

import (
	"errors"
	"os"
	"strings"
	"text/template"
	"time"

	"gofr.dev/pkg/gofr"
)

const (
	mig     = "migrations"
	allFile = "all.go"
)

var (
	errNameEmpty = errors.New(`please provide the name of the migration using "-name" option`)
)

//nolint:gochecknoglobals // keeping them local so that they are computed at the compile time.
var (
	allTemplate = template.Must(template.New("allContent").Parse(
		`// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate {
{{range $key, $value := .}}	
		{{ $key }}: {{ $value }}(),{{end}}
	}
}
`))

	migrationTemplate = template.Must(template.New("migrationContent").
				Parse(
			`package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func {{ . }}() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			// write your migrations here

			return nil
		},
	}
}
`))
)

func Migrate(ctx *gofr.Context) (interface{}, error) {
	migName := ctx.Param("name")
	if migName == "" {
		return nil, errNameEmpty
	}

	if err := createMigrationFile(ctx, migName); err != nil {
		return nil, err
	}

	if err := createAllMigration(ctx); err != nil {
		return nil, err
	}

	return "Successfully created migration " + migName, nil
}

func createMigrationFile(ctx *gofr.Context, migrationName string) error {
	if _, err := os.Stat(mig); os.IsNotExist(err) {
		er := ctx.File.MkdirAll(mig, os.ModePerm)
		if er != nil {
			return er
		}
	}

	if err := os.Chdir(mig); err != nil {
		return err
	}

	currTimeStamp := time.Now().Format("20060102150405")

	fileName := currTimeStamp + "_" + migrationName

	file, err := ctx.File.OpenFile(fileName+".go", os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}

	defer file.Close()

	err = migrationTemplate.Execute(file, migrationName)
	if err != nil {
		return err
	}

	return nil
}

func createAllMigration(ctx *gofr.Context) error {
	f, err := ctx.File.Create(allFile)
	if err != nil {
		return err
	}

	d, err := os.ReadDir("./")
	if err != nil {
		return err
	}

	migrations := findMigrations(d)

	err = allTemplate.Execute(f, migrations)
	if err != nil {
		return err
	}

	return nil
}

func findMigrations(files []os.DirEntry) map[string]string {
	var existingMig = make(map[string]string)

	for _, file := range files {
		fileParts := strings.Split(file.Name(), "_")
		if len(fileParts) < 2 || file.Name() == allFile || fileParts[len(fileParts)-1] == "test.go" {
			continue
		}

		existingMig[fileParts[0]] = strings.TrimSuffix(fileParts[1], ".go")
	}

	return existingMig
}
