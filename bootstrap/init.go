package bootstrap

import (
	"fmt"
	"os"
	"text/template"

	"gofr.dev/pkg/gofr"
)

const (
	modContent = `module {{ .Module }}

go 1.22.4

require gofr.dev v{{ .GofrVersion }}
`
	mainContent = `package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.GET("/hello", func(ctx *gofr.Context) (interface{}, error) {
		return "Hello World!", nil
	})

	app.Run()
}
`
	fileMode = 0644
)

type modInfo struct {
	Module      string
	GofrVersion string
}

func Create(ctx *gofr.Context) (interface{}, error) {
	name := ctx.Param("name")
	gofrVersion := ctx.Param("gofr")

	if gofrVersion == "" {
		gofrVersion = "1.15.0"
	}

	modFile, err := os.OpenFile("go.mod", os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, err
	}

	t, err := template.New("go-mod").Parse(modContent)
	if err != nil {
		return nil, err
	}

	err = t.Execute(modFile, modInfo{Module: name, GofrVersion: gofrVersion})
	if err != nil {
		return nil, err
	}

	fmt.Println("Note: Please do go mod tidy to sync the dependencies of your project")

	mainFile, err := os.OpenFile("main.go", os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, err
	}

	_, err = mainFile.WriteString(mainContent)
	if err != nil {
		return nil, err
	}

	return "Successfully initialized project " + name, nil
}
