package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gofr.dev/pkg/gofr"
	"gopkg.in/yaml.v3"
)

const (
	filePerm = 0644
)

var (
	ErrNoConfigFile     = errors.New("config file path is required")
	ErrOpeningConfigFile = errors.New("error opening the config file")
	ErrFailedToParseConfig = errors.New("failed to parse config file")
	ErrGeneratingStore  = errors.New("error while generating the store code")
	ErrWritingFile      = errors.New("error writing the generated code to the file")
)

// StoreConfig represents the YAML configuration for store generation
type StoreConfig struct {
	Version string     `yaml:"version"`
	Store   StoreInfo  `yaml:"store"`
	Models  []Model    `yaml:"models"`
	Queries []Query    `yaml:"queries"`
}

// StoreInfo contains store-level configuration
type StoreInfo struct {
	Package     string `yaml:"package"`
	OutputDir   string `yaml:"output_dir"`
	Interface   string `yaml:"interface"`
	Implementation string `yaml:"implementation"`
}

// Model represents a data model
type Model struct {
	Name   string `yaml:"name"`
	Fields []Field `yaml:"fields,omitempty"`
	Path   string `yaml:"path,omitempty"` // Path to existing model file
	Package string `yaml:"package,omitempty"` // Package name for imported model
}

// Field represents a model field
type Field struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Tag      string `yaml:"tag,omitempty"`
	Nullable bool   `yaml:"nullable,omitempty"`
}

// Query represents a database query
type Query struct {
	Name        string            `yaml:"name"`
	SQL         string            `yaml:"sql"`
	Type        string            `yaml:"type"` // select, insert, update, delete, transaction, health
	Model       string            `yaml:"model,omitempty"`
	Params      []QueryParam      `yaml:"params,omitempty"`
	Returns     string            `yaml:"returns,omitempty"` // single, multiple, count, health
	Description string            `yaml:"description,omitempty"`
	Tags        map[string]string `yaml:"tags,omitempty"`
	UseSelect   bool              `yaml:"use_select,omitempty"` // Use GoFr's Select method
	Transaction bool              `yaml:"transaction,omitempty"` // Wrap in transaction
}

// QueryParam represents a query parameter
type QueryParam struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

// InitStore creates the initial store structure and configuration
func InitStore(ctx *gofr.Context) (interface{}, error) {
	storeName := ctx.Param("name")
	if storeName == "" {
		return nil, errors.New("store name is required. Use: gofr store init -name=<store_name>")
	}

	// Create stores directory if it doesn't exist
	if err := os.MkdirAll("stores", 0755); err != nil {
		return nil, fmt.Errorf("failed to create stores directory: %w", err)
	}

	// Create store-specific directory
	storeDir := fmt.Sprintf("stores/%s", strings.ToLower(storeName))
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	// Generate store.yaml configuration file
	if err := generateStoreConfig(ctx, storeName, storeDir); err != nil {
		return nil, fmt.Errorf("failed to generate store config: %w", err)
	}

	// Generate initial interface.go
	if err := generateInitialInterface(ctx, storeName, storeDir); err != nil {
		return nil, fmt.Errorf("failed to generate initial interface: %w", err)
	}

	// Generate initial store.go
	if err := generateInitialStore(ctx, storeName, storeDir); err != nil {
		return nil, fmt.Errorf("failed to generate initial store: %w", err)
	}

	// Generate all.go
	if err := generateAllStore(ctx, storeName, storeDir); err != nil {
		return nil, fmt.Errorf("failed to generate all.go: %w", err)
	}

	ctx.Logger.Infof("Successfully initialized store: %s", storeName)
	return fmt.Sprintf("Successfully initialized store: %s", storeName), nil
}

// GenerateStore generates store layer functions based on YAML configuration
func GenerateStore(ctx *gofr.Context) (interface{}, error) {
	configPath := ctx.Param("config")
	if configPath == "" {
		// Default to stores/store.yaml if no config specified
		configPath = "stores/store.yaml"
	}

	config, err := parseConfigFile(ctx, configPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to parse config file: %v", err)
		return nil, err
	}

	outputDir := config.Store.OutputDir
	if outputDir == "" {
		outputDir = "stores"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate interface file
	if err := generateInterface(ctx, config, outputDir); err != nil {
		return nil, fmt.Errorf("failed to generate interface: %w", err)
	}

	// Generate implementation file
	if err := generateImplementation(ctx, config, outputDir); err != nil {
		return nil, fmt.Errorf("failed to generate implementation: %w", err)
	}

	// Generate model files
	if err := generateModels(ctx, config, outputDir); err != nil {
		return nil, fmt.Errorf("failed to generate models: %w", err)
	}

	ctx.Logger.Info("Successfully generated store layer files")

	return "Successfully generated store layer files", nil
}

// parseConfigFile opens and parses the YAML config file
func parseConfigFile(ctx *gofr.Context, configPath string) (*StoreConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to open config file: %v", err)
		return nil, ErrOpeningConfigFile
	}
	defer file.Close()

	var config StoreConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		ctx.Logger.Errorf("Failed to parse config file: %v", err)
		return nil, ErrFailedToParseConfig
	}

	// Set defaults
	if config.Store.Package == "" {
		config.Store.Package = "store"
	}
	if config.Store.Interface == "" {
		config.Store.Interface = "Store"
	}
	if config.Store.Implementation == "" {
		config.Store.Implementation = "store"
	}

	return &config, nil
}

// collectImports collects all required imports for the generated code
func collectImports(config *StoreConfig) []string {
	imports := []string{"gofr.dev/pkg/gofr"}
	importMap := make(map[string]bool)
	
	// Add imports for models that have external paths
	for _, model := range config.Models {
		if model.Path != "" && model.Package != "" {
			if !importMap[model.Package] {
				imports = append(imports, model.Package)
				importMap[model.Package] = true
			}
		}
	}
	
	return imports
}

// generateInterface generates the store interface file
func generateInterface(ctx *gofr.Context, config *StoreConfig, outputDir string) error {
	interfaceFile := filepath.Join(outputDir, "interface.go")
	imports := collectImports(config)
	
	tmpl := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .Store.Package }}

import (
{{range .Imports}}
	"{{ . }}"
{{end}}
)

// {{ .Store.Interface }} defines the interface for store operations
type {{ .Store.Interface }} interface {
{{range .Queries}}
	{{ .Name }}(ctx *gofr.Context{{range .Params}}, {{ .Name }} {{ .Type }}{{end}}) ({{ if eq .Returns "single" }}{{ .Model | getModelType }}, {{ else if eq .Returns "multiple" }}[]{{ .Model | getModelType }}, {{ else if eq .Returns "count" }}int64, {{ else }}interface{}, {{ end }}error)
{{end}}
}
`

	t, err := template.New("interface").Funcs(template.FuncMap{
		"getModelType": func(modelName string) string {
			// Check if this model is from an external package
			for _, model := range config.Models {
				if model.Name == modelName && model.Path != "" && model.Package != "" {
					// Extract package name from the full package path
					parts := strings.Split(model.Package, "/")
					pkgName := parts[len(parts)-1]
					return pkgName + "." + modelName
				}
			}
			return modelName
		},
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse interface template: %w", err)
	}

	file, err := os.Create(interfaceFile)
	if err != nil {
		return fmt.Errorf("failed to create interface file: %w", err)
	}
	defer file.Close()

	data := struct {
		*StoreConfig
		Imports []string
	}{config, imports}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute interface template: %w", err)
	}

	ctx.Logger.Infof("Generated interface file: %s", interfaceFile)
	return nil
}

// generateImplementation generates the store implementation file
func generateImplementation(ctx *gofr.Context, config *StoreConfig, outputDir string) error {
	implFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", config.Store.Implementation))
	imports := collectImports(config)
	
	tmpl := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .Store.Package }}

import (
{{range .Imports}}
	"{{ . }}"
{{end}}
)

// {{ .Store.Implementation }} implements the {{ .Store.Interface }} interface
type {{ .Store.Implementation }} struct {
	// Add any dependencies here (e.g., database connection)
}

// New{{ .Store.Interface }} creates a new instance of {{ .Store.Interface }}
func New{{ .Store.Interface }}() {{ .Store.Interface }} {
	return &{{ .Store.Implementation }}{}
}

{{range .Queries}}
// {{ .Name }} {{ if .Description }}{{ .Description }}{{ else }}executes the {{ .Name }} query{{ end }}
func (s *{{ $.Store.Implementation }}) {{ .Name }}(ctx *gofr.Context{{range .Params}}, {{ .Name }} {{ .Type }}{{end}}) ({{ if eq .Returns "single" }}{{ .Model | getModelType }}, {{ else if eq .Returns "multiple" }}[]{{ .Model | getModelType }}, {{ else if eq .Returns "count" }}int64, {{ else }}interface{}, {{ end }}error) {
	// TODO: Implement {{ .Name }} query
	// SQL: {{ .SQL }}
	
	{{ if eq .Type "select" }}
		{{ if eq .Returns "single" }}
			var result {{ .Model | getModelType }}
			// Implement single row selection using ctx.SQL()
			// Example: err := ctx.SQL().QueryRowContext(ctx, "{{ .SQL }}", {{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}}).Scan(&result.Field1, &result.Field2, ...)
			return result, nil
		{{ else if eq .Returns "multiple" }}
			var results []{{ .Model | getModelType }}
			// Implement multiple row selection using ctx.SQL()
			// Example: rows, err := ctx.SQL().QueryContext(ctx, "{{ .SQL }}", {{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}})
			return results, nil
		{{ else if eq .Returns "count" }}
			var count int64
			// Implement count query using ctx.SQL()
			// Example: err := ctx.SQL().QueryRowContext(ctx, "{{ .SQL }}", {{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}}).Scan(&count)
			return count, nil
		{{ else }}
			// Implement custom return type
			return nil, nil
		{{ end }}
	{{ else if eq .Type "insert" }}
		// Implement insert operation using ctx.SQL()
		// Example: result, err := ctx.SQL().ExecContext(ctx, "{{ .SQL }}", {{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}})
		return {{ if eq .Returns "single" }}{{ .Model }}{}, {{ else if eq .Returns "count" }}int64(0), {{ else }}nil, {{ end }}nil
	{{ else if eq .Type "update" }}
		// Implement update operation using ctx.SQL()
		// Example: result, err := ctx.SQL().ExecContext(ctx, "{{ .SQL }}", {{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}})
		return {{ if eq .Returns "count" }}int64(0), {{ else }}nil, {{ end }}nil
	{{ else if eq .Type "delete" }}
		// Implement delete operation using ctx.SQL()
		// Example: result, err := ctx.SQL().ExecContext(ctx, "{{ .SQL }}", {{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}})
		return {{ if eq .Returns "count" }}int64(0), {{ else }}nil, {{ end }}nil
	{{ end }}
}
{{end}}
`

	t, err := template.New("implementation").Funcs(template.FuncMap{
		"getModelType": func(modelName string) string {
			// Check if this model is from an external package
			for _, model := range config.Models {
				if model.Name == modelName && model.Path != "" && model.Package != "" {
					// Extract package name from the full package path
					parts := strings.Split(model.Package, "/")
					pkgName := parts[len(parts)-1]
					return pkgName + "." + modelName
				}
			}
			return modelName
		},
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse implementation template: %w", err)
	}

	file, err := os.Create(implFile)
	if err != nil {
		return fmt.Errorf("failed to create implementation file: %w", err)
	}
	defer file.Close()

	data := struct {
		*StoreConfig
		Imports []string
	}{config, imports}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute implementation template: %w", err)
	}

	ctx.Logger.Infof("Generated implementation file: %s", implFile)
	return nil
}

// generateModels generates model files or references existing ones
func generateModels(ctx *gofr.Context, config *StoreConfig, outputDir string) error {
	for _, model := range config.Models {
		// If model has a path, it's referencing an existing model file
		if model.Path != "" {
			ctx.Logger.Infof("Referencing existing model: %s from %s", model.Name, model.Path)
			continue
		}
		
		// Generate new model file only if no path is specified
		modelFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", strings.ToLower(model.Name)))
		
		tmpl := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ $.Store.Package }}

import (
	"time"
)

// {{ .Name }} represents the {{ .Name }} model
type {{ .Name }} struct {
{{range .Fields}}
	{{ .Name }} {{ .Type }} ` + "`" + `{{ .Tag }}` + "`" + `{{end}}
}

// TableName returns the table name for {{ .Name }}
func ({{ .Name }}) TableName() string {
	return "{{ .Name | lower }}"
}
`

		t, err := template.New("model").Funcs(template.FuncMap{
			"lower": strings.ToLower,
		}).Parse(tmpl)
		if err != nil {
			return fmt.Errorf("failed to parse model template: %w", err)
		}

		file, err := os.Create(modelFile)
		if err != nil {
			return fmt.Errorf("failed to create model file: %w", err)
		}
		defer file.Close()

		if err := t.Execute(file, struct {
			*StoreConfig
			Model
		}{config, model}); err != nil {
			return fmt.Errorf("failed to execute model template: %w", err)
		}

		ctx.Logger.Infof("Generated model file: %s", modelFile)
	}

	return nil
}

// generateStoreConfig creates the initial store.yaml configuration file
func generateStoreConfig(ctx *gofr.Context, storeName, storeDir string) error {
	configFile := filepath.Join(storeDir, "store.yaml")
	
	tmpl := `version: "1.0"

store:
  package: "{{ .PackageName }}"
  output_dir: "{{ .OutputDir }}"
  interface: "{{ .InterfaceName }}"
  implementation: "{{ .ImplementationName }}"

models:
  # Define your models here
  # Option 1: Generate new model struct
  # - name: "User"
  #   fields:
  #     - name: "ID"
  #       type: "int64"
  #       tag: "db:\"id\" json:\"id\""
  #     - name: "Name"
  #       type: "string"
  #       tag: "db:\"name\" json:\"name\""
  #
  # Option 2: Reference existing model from another package
  # - name: "User"
  #   path: "models/user.go"
  #   package: "test-store-project/models"

queries:
  # Define your queries here
  # Example:
  # - name: "GetUserByID"
  #   sql: "SELECT id, name FROM users WHERE id = ?"
  #   type: "select"
  #   model: "User"
  #   returns: "single"
  #   params:
  #     - name: "id"
  #       type: "int64"
  #   description: "Retrieves a user by their ID"
`

	t, err := template.New("config").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse config template: %w", err)
	}

	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	data := struct {
		PackageName        string
		OutputDir          string
		InterfaceName      string
		ImplementationName string
	}{
		PackageName:        strings.ToLower(storeName),
		OutputDir:          storeDir,
		InterfaceName:      strings.Title(storeName) + "Store",
		ImplementationName: strings.ToLower(storeName),
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	ctx.Logger.Infof("Generated config file: %s", configFile)
	return nil
}

// generateInitialInterface creates the initial interface.go file
func generateInitialInterface(ctx *gofr.Context, storeName, storeDir string) error {
	interfaceFile := filepath.Join(storeDir, "interface.go")
	
	tmpl := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .PackageName }}

import (
	"gofr.dev/pkg/gofr"
)

// {{ .InterfaceName }} defines the interface for {{ .StoreName }} store operations
type {{ .InterfaceName }} interface {
	// Add your store methods here
	// Example:
	// GetUserByID(ctx *gofr.Context, id int64) (User, error)
}
`

	t, err := template.New("interface").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse interface template: %w", err)
	}

	file, err := os.Create(interfaceFile)
	if err != nil {
		return fmt.Errorf("failed to create interface file: %w", err)
	}
	defer file.Close()

	data := struct {
		PackageName   string
		InterfaceName string
		StoreName     string
	}{
		PackageName:   strings.ToLower(storeName),
		InterfaceName: strings.Title(storeName) + "Store",
		StoreName:     storeName,
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute interface template: %w", err)
	}

	ctx.Logger.Infof("Generated interface file: %s", interfaceFile)
	return nil
}

// generateInitialStore creates the initial store.go file
func generateInitialStore(ctx *gofr.Context, storeName, storeDir string) error {
	storeFile := filepath.Join(storeDir, fmt.Sprintf("%s.go", strings.ToLower(storeName)))
	
	tmpl := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .PackageName }}

import (
	"gofr.dev/pkg/gofr"
)

// {{ .ImplementationName }} implements the {{ .InterfaceName }} interface
type {{ .ImplementationName }} struct {
	// Add any dependencies here (e.g., database connection)
}

// New{{ .InterfaceName }} creates a new instance of {{ .InterfaceName }}
func New{{ .InterfaceName }}() {{ .InterfaceName }} {
	return &{{ .ImplementationName }}{}
}

// Add your store method implementations here
// Example:
// func (s *{{ .ImplementationName }}) GetUserByID(ctx *gofr.Context, id int64) (User, error) {
//     // TODO: Implement GetUserByID query
//     var result User
//     err := ctx.SQL().QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = ?", id).Scan(&result.ID, &result.Name)
//     return result, err
// }
`

	t, err := template.New("store").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse store template: %w", err)
	}

	file, err := os.Create(storeFile)
	if err != nil {
		return fmt.Errorf("failed to create store file: %w", err)
	}
	defer file.Close()

	data := struct {
		PackageName      string
		ImplementationName string
		InterfaceName    string
	}{
		PackageName:        strings.ToLower(storeName),
		ImplementationName: strings.ToLower(storeName),
		InterfaceName:      strings.Title(storeName) + "Store",
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute store template: %w", err)
	}

	ctx.Logger.Infof("Generated store file: %s", storeFile)
	return nil
}

// generateAllStore creates the all.go file similar to migrations
func generateAllStore(ctx *gofr.Context, storeName, storeDir string) error {
	allFile := filepath.Join(storeDir, "all.go")
	
	tmpl := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .PackageName }}

import (
	"gofr.dev/pkg/gofr"
)

// All returns all available store implementations
func All() map[string]func() interface{} {
	return map[string]func() interface{} {
		"{{ .StoreName }}": func() interface{} {
			return New{{ .InterfaceName }}()
		},
	}
}

// GetStore returns a specific store by name
func GetStore(name string) interface{} {
	stores := All()
	if storeFunc, exists := stores[name]; exists {
		return storeFunc()
	}
	return nil
}
`

	t, err := template.New("all").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse all template: %w", err)
	}

	file, err := os.Create(allFile)
	if err != nil {
		return fmt.Errorf("failed to create all file: %w", err)
	}
	defer file.Close()

	data := struct {
		PackageName   string
		StoreName     string
		InterfaceName string
	}{
		PackageName:   strings.ToLower(storeName),
		StoreName:     storeName,
		InterfaceName: strings.Title(storeName) + "Store",
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute all template: %w", err)
	}

	ctx.Logger.Infof("Generated all file: %s", allFile)
	return nil
}
