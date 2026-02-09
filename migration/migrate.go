package migration

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"gofr.dev/pkg/gofr"
)

const (
	mig         = "migrations"
	allFile     = "all.go"
	matchLength = 3
)

var (
	errNameEmpty    = errors.New(`please provide the name of the migration using "-name" option`)
	errScanningFile = errors.New("failed to scan existing all.go file")
	migRegex        = regexp.MustCompile(`^\s*(\d+)\s*:\s*([a-zA-Z_]+)\(\),?\s*$`)
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

	migrationTemplate = template.Must(template.New("migrationContent").Parse(
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

func Migrate(ctx *gofr.Context) (any, error) {
	migName := ctx.Param("name")
	if migName == "" {
		return nil, errNameEmpty
	}

	camelCasedMigName := toCamelCase(migName)

	if err := createMigrationFile(ctx, camelCasedMigName); err != nil {
		return nil, fmt.Errorf("error while creating migration file, err: %w", err)
	}

	if err := createAllMigration(ctx); err != nil {
		return nil, fmt.Errorf("error while creating all.go file, err: %w", err)
	}

	return fmt.Sprintf("Successfully created migration %v", camelCasedMigName), nil
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

	defer func() {
		_ = file.Close()
	}()

	err = migrationTemplate.Execute(file, migrationName)
	if err != nil {
		return err
	}

	return nil
}

func createAllMigration(ctx *gofr.Context) error {
	existing := make(map[string]string)

	existing, err := getAllExistingMigrations(ctx, existing)
	if err != nil {
		return err
	}

	f, err := ctx.File.Create(allFile)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	d, err := os.ReadDir("./")
	if err != nil {
		return err
	}

	currentMigs := findMigrations(d)

	// Merge new migrations into existing map
	for ts, fn := range currentMigs {
		if _, ok := existing[ts]; !ok {
			existing[ts] = fn
		}
	}

	err = allTemplate.Execute(f, existing)
	if err != nil {
		return err
	}

	return nil
}

func getAllExistingMigrations(ctx *gofr.Context, existing map[string]string) (map[string]string, error) {
	if _, err := os.Stat(allFile); err == nil {
		file, err := ctx.File.OpenFile(allFile, os.O_RDONLY, os.ModePerm)
		if err != nil {
			return nil, err
		}

		defer func() {
			_ = file.Close()
		}()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			matches := migRegex.FindStringSubmatch(line)
			if len(matches) == matchLength {
				timestamp := matches[1]
				funcName := matches[2]
				existing[timestamp] = funcName
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("%w: %w", errScanningFile, err)
		}
	}

	return existing, nil
}

func findMigrations(files []os.DirEntry) map[string]string {
	var existingMig = make(map[string]string)

	for _, file := range files {
		fileParts := strings.Split(file.Name(), "_")
		if len(fileParts) < 2 || file.Name() == allFile || fileParts[len(fileParts)-1] == "test.go" {
			continue
		}
		// convert second part (function) to camelCase, since migration files are now generated using camelCase
		existingMig[fileParts[0]] = toCamelCase(strings.TrimSuffix(strings.Join(fileParts[1:], "_"), ".go"))
	}

	return existingMig
}

// toCamelCase converts snake_case or kebab-case to camelCase. If input has no delimiters,
// it preserves existing casing except lowercasing the first letter.
func toCamelCase(migrationName string) string {
	if migrationName == "" {
		return migrationName
	}
	// If no delimiters, assume it's already camel/Pascal case or a single word; just lowercase first rune
	if !strings.Contains(migrationName, "_") && !strings.Contains(migrationName, "-") {
		return strings.ToLower(migrationName[:1]) + migrationName[1:]
	}

	migrationName = strings.ReplaceAll(migrationName, "-", "_")
	parts := strings.FieldsFunc(migrationName, func(r rune) bool { return r == '_' || r == '-' })

	if len(parts) == 0 {
		return ""
	}

	result := strings.ToLower(parts[0])

	for _, part := range parts[1:] {
		if part != "" {
			result += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}

	return result
}
