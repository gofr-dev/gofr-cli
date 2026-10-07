package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
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
	"{{ .Module }}/handler"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	h := handler.New()

	app.GET("/hello", h.Hello)

	app.Run()
}
`
	handlerContent = `package handler

import (
	"{{ .Module }}/service"

	"gofr.dev/pkg/gofr"
)

type Handler struct {
	Service *service.Service
}

func New() *Handler {
	return &Handler{
		Service: service.New(),
	}
}

func (h *Handler) Hello(ctx *gofr.Context) (any, error) {
	msg, err := h.Service.Hello(ctx)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
`
	serviceContent = `package service

import (
	"{{ .Module }}/store"

	"gofr.dev/pkg/gofr"
)

type Service struct {
	Store *store.Store
}

func New() *Service {
	return &Service{
		Store: store.New(),
	}
}

func (s *Service) Hello(ctx *gofr.Context) (string, error) {
	return s.Store.Hello(ctx)
}
`
	storeContent = `package store

import (
	"gofr.dev/pkg/gofr"
)

type Store struct{}

func New() *Store {
	return &Store{}
}

func (s *Store) Hello(ctx *gofr.Context) (string, error) {
	return "Hello from Store!", nil
}
`
	fileMode     = 0644
	dirFileMode  = 0755
)

type modInfo struct {
	Module      string
	GofrVersion string
}

func Create(ctx *gofr.Context) (any, error) {
	name := ctx.Param("name")
	gofrVersion := ctx.Param("gofr")

	if gofrVersion == "" {
		gofrVersion = "1.17.0"
	}

	info := modInfo{Module: name, GofrVersion: gofrVersion}

	if err := os.MkdirAll("handler", dirFileMode); err != nil {
		return nil, err
	}

	if err := os.MkdirAll("service", dirFileMode); err != nil {
		return nil, err
	}

	if err := os.MkdirAll("store", dirFileMode); err != nil {
		return nil, err
	}

	modFile, err := os.OpenFile("go.mod", os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, err
	}

	t, err := template.New("go-mod").Parse(modContent)
	if err != nil {
		return nil, err
	}

	err = t.Execute(modFile, info)
	if err != nil {
		return nil, err
	}

	fmt.Println("Note: Please do go mod tidy to sync the dependencies of your project")

	if err := writeTemplateFile("main.go", mainContent, info); err != nil {
		return nil, err
	}

	if err := writeTemplateFile(filepath.Join("handler", "handler.go"), handlerContent, info); err != nil {
		return nil, err
	}

	if err := writeTemplateFile(filepath.Join("service", "service.go"), serviceContent, info); err != nil {
		return nil, err
	}

	if err := writeTemplateFile(filepath.Join("store", "store.go"), storeContent, info); err != nil {
		return nil, err
	}

	return "Successfully initialized project " + name + " with three-layer architecture (handler, service, store)", nil
}

func writeTemplateFile(path, content string, info modInfo) error {
	t, err := template.New(filepath.Base(path)).Parse(content)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, info)
}
