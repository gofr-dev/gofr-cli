package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"gofr.dev/pkg/gofr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

const (
	defaultFilePerm    = 0644
	defaultDirPerm     = 0755
	defaultPackage     = "store"
	allStoresFile      = "stores/all.go"
	minMatchLength     = 2 // Minimum regex match length
	minPartsLength     = 2 // Minimum parts length for module detection
	linesPerStoreEntry = 3 // Number of lines per store entry in all.go
)

var (
	errStoreNameRequired   = errors.New("store name is required. Use: gofr store init -name=<store_name>")
	errNoStoresDefined     = errors.New("no stores defined in configuration")
	errOpeningConfigFile   = errors.New("error opening the config file")
	errFailedToParseConfig = errors.New("failed to parse config file")
)

// storeRegex matches store entries in all.go file.
var storeRegex = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*func\(\)\s*any\s*\{`)

// Config represents the YAML configuration for store generation.
type Config struct {
	Version string  `yaml:"version"`
	Stores  []Info  `yaml:"stores"`
	Models  []Model `yaml:"models"`
}

// Info contains store-level configuration.
type Info struct {
	Name           string  `yaml:"name"`
	Package        string  `yaml:"package"`
	OutputDir      string  `yaml:"output_dir"`
	Interface      string  `yaml:"interface"`
	Implementation string  `yaml:"implementation"`
	Queries        []Query `yaml:"queries"`
}

// Model represents a data model.
type Model struct {
	Name    string  `yaml:"name"`
	Fields  []Field `yaml:"fields,omitempty"`
	Path    string  `yaml:"path,omitempty"`    // Path to existing model file
	Package string  `yaml:"package,omitempty"` // Package name for imported model
}

// Field represents a model field.
type Field struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Tag      string `yaml:"tag,omitempty"`
	Nullable bool   `yaml:"nullable,omitempty"`
}

// Query represents a database query.
type Query struct {
	Name        string            `yaml:"name"`
	SQL         string            `yaml:"sql"`
	Type        string            `yaml:"type"` // select, insert, update, delete, transaction, health
	Model       string            `yaml:"model,omitempty"`
	Params      []QueryParam      `yaml:"params,omitempty"`
	Returns     string            `yaml:"returns,omitempty"` // single, multiple, count, health
	Description string            `yaml:"description,omitempty"`
	Tags        map[string]string `yaml:"tags,omitempty"`
	UseSelect   bool              `yaml:"use_select,omitempty"`
	Transaction bool              `yaml:"transaction,omitempty"`
}

// QueryParam represents a query parameter.
type QueryParam struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

// Entry represents a store entry for the all.go registry.
type Entry struct {
	Name          string
	PackageName   string
	InterfaceName string
}

// InitStore creates the initial store structure and configuration.
func InitStore(ctx *gofr.Context) (any, error) {
	storeName := ctx.Param("name")
	if storeName == "" {
		return nil, errStoreNameRequired
	}

	// Create stores directory if it doesn't exist
	if err := os.MkdirAll("stores", defaultDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create stores directory: %w", err)
	}

	// Create store-specific directory
	storeDir := fmt.Sprintf("stores/%s", strings.ToLower(storeName))
	if err := os.MkdirAll(storeDir, defaultDirPerm); err != nil {
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

	// Generate/update all.go at stores root level (with proper appending)
	newStores := []Entry{{
		Name:          storeName,
		PackageName:   strings.ToLower(storeName),
		InterfaceName: cases.Title(language.English).String(storeName) + "Store",
	}}

	if err := appendStoreEntries(ctx, newStores); err != nil {
		ctx.Logger.Errorf("Failed to update all.go: %v", err)
		return nil, fmt.Errorf("failed to update all.go: %w", err)
	}

	ctx.Logger.Infof("Successfully initialized store: %s", storeName)

	return fmt.Sprintf("Successfully initialized store: %s", storeName), nil
}

// GenerateStore generates store layer functions based on YAML configuration.
func GenerateStore(ctx *gofr.Context) (any, error) {
	configPath := ctx.Param("config")
	if configPath == "" {
		configPath = "stores/store.yaml"
	}

	config, err := parseConfigFile(ctx, configPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to parse config file: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Parsed config with %d stores", len(config.Stores))

	if len(config.Stores) == 0 {
		return nil, errNoStoresDefined
	}

	// Generate each store
	for _, store := range config.Stores {
		if err := generateSingleStore(ctx, config, &store); err != nil {
			return nil, fmt.Errorf("failed to generate store %s: %w", store.Name, err)
		}
	}

	// Convert stores to Entry format
	newStores := make([]Entry, 0, len(config.Stores))
	for _, store := range config.Stores {
		newStores = append(newStores, Entry{
			Name:          store.Name,
			PackageName:   strings.ToLower(store.Name),
			InterfaceName: cases.Title(language.English).String(store.Name) + "Store",
		})
	}

	// Update all.go at the stores root level (append mode)
	ctx.Logger.Infof("About to update all.go with %d stores", len(newStores))

	if err := appendStoreEntries(ctx, newStores); err != nil {
		return nil, fmt.Errorf("failed to update all.go: %w", err)
	}

	ctx.Logger.Info("Successfully generated store layer files")

	return "Successfully generated store layer files", nil
}

// generateSingleStore generates a single store.
func generateSingleStore(ctx *gofr.Context, config *Config, store *Info) error {
	outputDir := store.OutputDir
	if outputDir == "" {
		outputDir = fmt.Sprintf("stores/%s", store.Name)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, defaultDirPerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create a store-specific config for this store
	storeConfig := &Config{
		Version: config.Version,
		Models:  config.Models,
		Stores:  []Info{*store},
	}

	// Generate interface file
	if err := generateInterface(ctx, storeConfig, outputDir); err != nil {
		return fmt.Errorf("failed to generate interface: %w", err)
	}

	// Generate implementation file
	if err := generateImplementation(ctx, storeConfig, outputDir); err != nil {
		return fmt.Errorf("failed to generate implementation: %w", err)
	}

	// Generate model files
	if err := generateModels(ctx, storeConfig, outputDir); err != nil {
		return fmt.Errorf("failed to generate models: %w", err)
	}

	ctx.Logger.Infof("Generated store: %s in %s", store.Name, outputDir)

	return nil
}

// parseConfigFile opens and parses the YAML config file.
func parseConfigFile(ctx *gofr.Context, configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to open config file: %v", err)
		return nil, errOpeningConfigFile
	}
	defer file.Close()

	var config Config

	decoder := yaml.NewDecoder(file)

	if err := decoder.Decode(&config); err != nil {
		ctx.Logger.Errorf("Failed to parse config file: %v", err)
		return nil, errFailedToParseConfig
	}

	return &config, nil
}

// collectImports collects all required imports for the generated code.
func collectImports(config *Config) []string {
	imports := []string{"gofr.dev/pkg/gofr"}
	importMap := make(map[string]bool)

	// Get models used by this specific store
	usedModels := getModelsUsedByStore(config)

	// Add imports for models that have external paths and are used by this store
	for _, model := range config.Models {
		if model.Path != "" && model.Package != "" && usedModels[model.Name] {
			if !importMap[model.Package] {
				imports = append(imports, model.Package)
				importMap[model.Package] = true
			}
		}
	}

	return imports
}

// getModelsUsedByStore returns a map of model names that are used by the current store.
func getModelsUsedByStore(config *Config) map[string]bool {
	usedModels := make(map[string]bool)

	// Check all queries in the current store (first store in the array)
	if len(config.Stores) > 0 {
		for i := range config.Stores[0].Queries {
			query := &config.Stores[0].Queries[i]
			if query.Model != "" {
				usedModels[query.Model] = true
			}
		}
	}

	return usedModels
}

// generateInterface generates the store interface file.
func generateInterface(ctx *gofr.Context, config *Config, outputDir string) error {
	interfaceFile := filepath.Join(outputDir, "interface.go")
	imports := collectImports(config)

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
	}).Parse(InterfaceTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse interface template: %w", err)
	}

	file, err := os.Create(interfaceFile)
	if err != nil {
		return fmt.Errorf("failed to create interface file: %w", err)
	}
	defer file.Close()

	data := struct {
		Store   Info
		Imports []string
	}{config.Stores[0], imports}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute interface template: %w", err)
	}

	ctx.Logger.Infof("Generated interface file: %s", interfaceFile)

	return nil
}

// generateImplementation generates the store implementation file.
func generateImplementation(ctx *gofr.Context, config *Config, outputDir string) error {
	implFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", config.Stores[0].Implementation))
	imports := collectImports(config)

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
	}).Parse(ImplementationTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse implementation template: %w", err)
	}

	file, err := os.Create(implFile)
	if err != nil {
		return fmt.Errorf("failed to create implementation file: %w", err)
	}
	defer file.Close()

	data := struct {
		Store   Info
		Imports []string
	}{config.Stores[0], imports}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute implementation template: %w", err)
	}

	ctx.Logger.Infof("Generated implementation file: %s", implFile)

	return nil
}

// generateModels generates model files or references existing ones.
func generateModels(ctx *gofr.Context, config *Config, outputDir string) error {
	// Get models used by this specific store
	usedModels := getModelsUsedByStore(config)

	for _, model := range config.Models {
		// Only generate models that are actually used by this store
		if !usedModels[model.Name] {
			continue
		}

		// If model has a path, it's referencing an existing model file
		if model.Path != "" {
			ctx.Logger.Infof("Referencing existing model: %s from %s", model.Name, model.Path)
			continue
		}

		// Generate new model file only if no path is specified
		modelFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", strings.ToLower(model.Name)))

		t, err := template.New("model").Funcs(template.FuncMap{
			"lower": strings.ToLower,
		}).Parse(ModelTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse model template: %w", err)
		}

		file, err := os.Create(modelFile)
		if err != nil {
			return fmt.Errorf("failed to create model file: %w", err)
		}

		// Pass the store and model context correctly
		store := config.Stores[0]
		if err := t.Execute(file, struct {
			Store Info
			Model Model
		}{store, model}); err != nil {
			file.Close() // Close file before returning error
			return fmt.Errorf("failed to execute model template: %w", err)
		}

		file.Close() // Close file explicitly instead of defer in loop
		ctx.Logger.Infof("Generated model file: %s", modelFile)
	}

	return nil
}

// generateStoreConfig creates the initial store.yaml configuration file.
func generateStoreConfig(ctx *gofr.Context, storeName, storeDir string) error {
	configFile := filepath.Join(storeDir, "store.yaml")

	t, err := template.New("config").Parse(StoreConfigTemplate)
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
		InterfaceName:      cases.Title(language.English).String(storeName) + "Store",
		ImplementationName: strings.ToLower(storeName),
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	ctx.Logger.Infof("Generated config file: %s", configFile)

	return nil
}

// generateInitialInterface creates the initial interface.go file with commented GoFr imports.
func generateInitialInterface(ctx *gofr.Context, storeName, storeDir string) error {
	interfaceFile := filepath.Join(storeDir, "interface.go")

	t, err := template.New("interface").Parse(InitialInterfaceTemplate)
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
		InterfaceName: cases.Title(language.English).String(storeName) + "Store",
		StoreName:     storeName,
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute interface template: %w", err)
	}

	ctx.Logger.Infof("Generated interface file: %s", interfaceFile)

	return nil
}

// generateInitialStore creates the initial store.go file with commented GoFr imports.
func generateInitialStore(ctx *gofr.Context, storeName, storeDir string) error {
	storeFile := filepath.Join(storeDir, fmt.Sprintf("%s.go", strings.ToLower(storeName)))

	t, err := template.New("store").Parse(InitialStoreTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse store template: %w", err)
	}

	file, err := os.Create(storeFile)
	if err != nil {
		return fmt.Errorf("failed to create store file: %w", err)
	}
	defer file.Close()

	data := struct {
		PackageName        string
		ImplementationName string
		InterfaceName      string
	}{
		PackageName:        strings.ToLower(storeName),
		ImplementationName: strings.ToLower(storeName),
		InterfaceName:      cases.Title(language.English).String(storeName) + "Store",
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute store template: %w", err)
	}

	ctx.Logger.Infof("Generated store file: %s", storeFile)

	return nil
}

// appendStoreEntries appends new stores to stores/all.go without overwriting existing entries.
func appendStoreEntries(ctx *gofr.Context, newStores []Entry) error {
	projectModule := detectProjectModule()
	if projectModule == "" {
		projectModule = "your-project"
	}

	// Read existing file
	content, err := os.ReadFile(allStoresFile)
	if err != nil {
		// If file doesn't exist, generate complete file
		return generateCompleteAllFile(ctx, newStores, projectModule)
	}

	return processExistingAllFile(ctx, content, newStores, projectModule)
}

// processExistingAllFile handles the logic for updating an existing all.go file.
func processExistingAllFile(ctx *gofr.Context, content []byte,
	newStores []Entry, projectModule string) error {
	lines := strings.Split(string(content), "\n")

	// Parse existing stores and imports more carefully
	existingStores, existingImports := parseExistingAllFile(lines)

	// Filter out stores that already exist and collect imports to add
	storesToAdd, importsToAdd := filterNewStores(newStores,
		existingStores, existingImports, projectModule)

	if len(storesToAdd) == 0 {
		ctx.Logger.Info("All stores already exist in all.go")

		return nil
	}

	return updateAllFileWithNewStores(ctx, lines, storesToAdd,
		importsToAdd, existingStores, projectModule)
}

// filterNewStores filters out stores that already exist and prepares imports to add.
func filterNewStores(newStores []Entry, existingStores, existingImports map[string]bool,
	projectModule string) (filteredStores []Entry, importsToAdd []string) {
	filteredStores = make([]Entry, 0, len(newStores))
	importsToAdd = make([]string, 0, len(newStores))

	for _, store := range newStores {
		if !existingStores[store.Name] {
			filteredStores = append(filteredStores, store)
			// Always add import for new stores
			importPath := fmt.Sprintf(`    "%s/stores/%s"`, projectModule, store.PackageName)
			if !existingImports[importPath] {
				importsToAdd = append(importsToAdd, importPath)
			}
		}
	}

	return filteredStores, importsToAdd
}

// updateAllFileWithNewStores updates the all.go file with new stores and imports.
func updateAllFileWithNewStores(ctx *gofr.Context, lines []string,
	storesToAdd []Entry, importsToAdd []string,
	existingStores map[string]bool, projectModule string) error {
	// Handle import section
	lines = handleImportSection(lines, importsToAdd)

	// Find map insertion point (try multiple strategies)
	mapInsertIdx := findMapInsertionPoint(lines)
	if mapInsertIdx == -1 {
		// Try alternative method
		mapInsertIdx = findMapInsertionPointAlternative(lines)
	}

	if mapInsertIdx == -1 {
		// Last resort: regenerate the entire file
		ctx.Logger.Warn("Could not find insertion point, regenerating entire all.go file")
		return regenerateCompleteAllFile(ctx, existingStores, storesToAdd, projectModule)
	}

	// Insert store entries
	storeEntries := make([]string, 0, len(storesToAdd)*linesPerStoreEntry)
	for _, store := range storesToAdd {
		storeEntries = append(storeEntries,
			fmt.Sprintf(`        %q: func() any {`, store.Name),
			fmt.Sprintf(`            return %s.New%s()`, store.PackageName, store.InterfaceName),
			`        },`)
	}

	// Insert store entries before the closing brace of the map
	lines = insertLines(lines, mapInsertIdx, storeEntries)

	// Write updated content
	updatedContent := strings.Join(lines, "\n")

	err := os.WriteFile(allStoresFile, []byte(updatedContent), defaultFilePerm)
	if err != nil {
		return fmt.Errorf("failed to write updated all.go: %w", err)
	}

	ctx.Logger.Infof("Appended %d new stores to all.go with their imports", len(storesToAdd))

	return nil
}

// regenerateCompleteAllFile regenerates the complete all.go file when insertion point cannot be found.
func regenerateCompleteAllFile(ctx *gofr.Context, existingStores map[string]bool,
	storesToAdd []Entry, projectModule string) error {
	// Combine existing and new stores
	allStores := make([]Entry, 0, len(existingStores)+len(storesToAdd))

	for storeName := range existingStores {
		// Reconstruct store entry from name (this is a fallback)
		allStores = append(allStores, Entry{
			Name:          storeName,
			PackageName:   storeName, // Assume package name matches store name
			InterfaceName: cases.Title(language.English).String(storeName) + "Store",
		})
	}

	allStores = append(allStores, storesToAdd...)

	return generateCompleteAllFile(ctx, allStores, projectModule)
}

// handleImportSection adds import section if missing or appends to existing one.
func handleImportSection(lines, importsToAdd []string) []string {
	if len(importsToAdd) == 0 {
		return lines
	}

	importInsertIdx := findImportInsertionPoint(lines)

	if importInsertIdx > 0 {
		// Import section exists, add to it
		return insertLines(lines, importInsertIdx, importsToAdd)
	}

	// No import section exists, create one
	return createImportSection(lines, importsToAdd)
}

// createImportSection creates a new import section in the file.
func createImportSection(lines, importsToAdd []string) []string {
	// Find where to insert import section (after package declaration)
	insertIdx := -1

	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			insertIdx = i + 1
			break
		}
	}

	if insertIdx == -1 {
		// Fallback: insert after first line
		insertIdx = 1
	}

	// Create import section
	importSection := []string{""}
	if len(importsToAdd) > 0 {
		importSection = append(importSection, "import (")
		importSection = append(importSection, importsToAdd...)
		importSection = append(importSection, ")")
	}

	return insertLines(lines, insertIdx, importSection)
}

// parseExistingAllFile parses the existing all.go file to extract stores and imports.
func parseExistingAllFile(lines []string) (existingStores, existingImports map[string]bool) {
	existingStores = make(map[string]bool)
	existingImports = make(map[string]bool)

	inImportSection := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for import section
		if strings.Contains(trimmedLine, "import (") {
			inImportSection = true
			continue
		}

		if inImportSection {
			if trimmedLine == ")" {
				inImportSection = false

				continue
			}
			// Extract import path
			if strings.Contains(trimmedLine, `"`) {
				existingImports[strings.TrimSpace(trimmedLine)] = true
			}

			continue
		}

		// Extract store names using regex
		matches := storeRegex.FindStringSubmatch(line)
		if len(matches) >= minMatchLength {
			existingStores[matches[1]] = true
		}
	}

	return existingStores, existingImports
}

// findImportInsertionPoint finds where to insert new import statements.
func findImportInsertionPoint(lines []string) int {
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// Find the closing parenthesis of import section
		if trimmedLine == ")" {
			// Check if this is actually the import section closing
			for j := i - 1; j >= 0; j-- {
				if strings.Contains(lines[j], "import (") {
					return i
				}
			}
		}
	}

	return -1
}

// findMapInsertionPoint finds where to insert new store entries in the map.
func findMapInsertionPoint(lines []string) int {
	mapStartFound := false
	braceDepth := 0

	for i, line := range lines {
		if shouldStartMapTracking(line) {
			mapStartFound = true
		}

		if mapStartFound {
			braceDepth = updateBraceDepth(line, braceDepth)

			if isMapClosingBrace(line, braceDepth) {
				return i
			}
		}
	}

	return -1
}

// shouldStartMapTracking determines if we should start tracking the map.
func shouldStartMapTracking(line string) bool {
	return strings.Contains(line, "func All()") ||
		strings.Contains(line, "return map[string]func() any") ||
		strings.Contains(line, "return map[string]func()any")
}

// updateBraceDepth updates the brace depth counter.
func updateBraceDepth(line string, currentDepth int) int {
	openBraces := strings.Count(line, "{")
	closeBraces := strings.Count(line, "}")

	return currentDepth + openBraces - closeBraces
}

// isMapClosingBrace checks if the current line is the map's closing brace.
func isMapClosingBrace(line string, braceDepth int) bool {
	trimmedLine := strings.TrimSpace(line)

	return braceDepth > 0 && (trimmedLine == "}" ||
		(strings.HasSuffix(trimmedLine, "}") &&
			!strings.Contains(trimmedLine, "{") &&
			!strings.Contains(trimmedLine, "func")))
}

// findMapInsertionPointAlternative is an alternative method for finding the map insertion point.
func findMapInsertionPointAlternative(lines []string) int {
	inAllFunction := false
	inMapReturn := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect start of All() function
		if strings.Contains(line, "func All()") {
			inAllFunction = true
			continue
		}

		// Detect map return statement
		if inAllFunction && (strings.Contains(line, "return map[string]func() any") ||
			strings.Contains(line, "return map[string]func()any")) {
			inMapReturn = true
			continue
		}

		// If we're in the map return, look for the closing brace
		if inMapReturn {
			// Look for standalone closing brace or closing brace with minimal content
			if trimmedLine == "}" ||
				(strings.HasPrefix(trimmedLine, "}") && len(trimmedLine) <= 3) {
				return i
			}
		}
	}

	return -1
}

// insertLines inserts new lines at the specified index.
func insertLines(lines []string, insertIdx int, newLines []string) []string {
	if insertIdx < 0 || insertIdx > len(lines) {
		return lines
	}

	result := make([]string, 0, len(lines)+len(newLines))
	result = append(result, lines[:insertIdx]...)
	result = append(result, newLines...)
	result = append(result, lines[insertIdx:]...)

	return result
}

// generateCompleteAllFile generates a complete all.go file from scratch.
func generateCompleteAllFile(ctx *gofr.Context, stores []Entry, projectModule string) error {
	// Create stores directory if it doesn't exist
	if err := os.MkdirAll("stores", defaultDirPerm); err != nil {
		return fmt.Errorf("failed to create stores directory: %w", err)
	}

	tmpl, err := template.New("all").Parse(AllStoresTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse all.go template: %w", err)
	}

	var buf bytes.Buffer

	data := struct {
		Stores        []Entry
		ProjectModule string
	}{
		Stores:        stores,
		ProjectModule: projectModule,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute all.go template: %w", err)
	}

	if err := os.WriteFile(allStoresFile, buf.Bytes(), defaultFilePerm); err != nil {
		return fmt.Errorf("failed to write all.go file: %w", err)
	}

	ctx.Logger.Infof("Generated complete all.go file: %s", allStoresFile)

	return nil
}

// detectProjectModule reads go.mod to determine the project module name.
func detectProjectModule() string {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= minPartsLength {
				return parts[1]
			}
		}
	}

	return ""
}
