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
	"unicode"

	"gofr.dev/pkg/gofr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

const (
	defaultFilePerm    = 0600
	defaultDirPerm     = 0755
	defaultPackage     = "store"
	allStoresFile      = "stores/all.go"
	minMatchLength     = 2
	minPartsLength     = 2
	linesPerStoreEntry = 3
	allFunctionName    = "All"
	stringType         = "string"
)

var (
	errStoreNameRequired   = errors.New("store name is required. Use: gofr store init -name=<store_name>")
	errNoStoresDefined     = errors.New("no stores defined in configuration")
	errOpeningConfigFile   = errors.New("error opening the config file")
	errFailedToParseConfig = errors.New("failed to parse config file")
	errInvalidStoreName    = errors.New("store name must be a valid Go identifier")
	errGoKeyword           = errors.New("cannot use Go keyword as name")
	errEmptyStoreName      = errors.New("store name cannot be empty")
	errEmptyPackageName    = errors.New("package name cannot be empty")
	errInvalidIdentifier   = errors.New("identifier must start with letter or underscore")
)

var storeRegex = regexp.MustCompile(`(?m)\s*"([^"]+)"\s*:\s*func\s*\(\s*\)\s*any\s*\{`)

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
	Path    string  `yaml:"path,omitempty"`
	Package string  `yaml:"package,omitempty"`
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
	Type        string            `yaml:"type"`
	Model       string            `yaml:"model,omitempty"`
	Params      []QueryParam      `yaml:"params,omitempty"`
	Returns     string            `yaml:"returns,omitempty"`
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

// ImportInfo represents an import with its path and optional alias.
type ImportInfo struct {
	Path  string
	Alias string
}

// ModelAliasMap maps model names to their import aliases for type resolution.
type ModelAliasMap map[string]string

// InitStore creates the initial store structure and configuration.
func InitStore(ctx *gofr.Context) (any, error) {
	storeName := ctx.Param("name")
	if storeName == "" {
		return nil, errStoreNameRequired
	}

	if err := validateGoIdentifier(storeName, "store name"); err != nil {
		return nil, fmt.Errorf("invalid store name: %w", err)
	}

	if err := os.MkdirAll("stores", defaultDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create stores directory: %w", err)
	}

	storeDir := fmt.Sprintf("stores/%s", strings.ToLower(storeName))
	if err := os.MkdirAll(storeDir, defaultDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	if err := generateStoreConfig(ctx, storeName, "stores"); err != nil {
		return nil, fmt.Errorf("failed to generate store config: %w", err)
	}

	if err := generateInitialInterface(ctx, storeName, storeDir); err != nil {
		return nil, fmt.Errorf("failed to generate initial interface: %w", err)
	}

	if err := generateInitialStore(ctx, storeName, storeDir); err != nil {
		return nil, fmt.Errorf("failed to generate initial store: %w", err)
	}

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

	cfg, err := parseConfigFile(ctx, configPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to parse config file: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Parsed config with %d stores", len(cfg.Stores))

	if len(cfg.Stores) == 0 {
		return nil, errNoStoresDefined
	}

	for i := range cfg.Stores {
		if err := generateSingleStore(ctx, cfg, &cfg.Stores[i]); err != nil {
			return nil, fmt.Errorf("failed to generate store %s: %w", cfg.Stores[i].Name, err)
		}
	}

	newStores := make([]Entry, 0, len(cfg.Stores))
	for i := range cfg.Stores {
		interfaceName := cfg.Stores[i].Interface
		if interfaceName == "" {
			interfaceName = cases.Title(language.English).String(cfg.Stores[i].Name) + "Store"
		}

		newStores = append(newStores, Entry{
			Name:          cfg.Stores[i].Name,
			PackageName:   cfg.Stores[i].Package,
			InterfaceName: interfaceName,
		})
	}

	ctx.Logger.Infof("About to update all.go with %d stores", len(newStores))

	if err := appendStoreEntries(ctx, newStores); err != nil {
		return nil, fmt.Errorf("failed to update all.go: %w", err)
	}

	ctx.Logger.Info("Successfully generated store layer files")

	return "Successfully generated store layer files", nil
}

// validateGoIdentifier validates that a string is a valid Go identifier.
func validateGoIdentifier(name, fieldName string) error {
	if name == "" {
		return fmt.Errorf("%w: %s", errEmptyStoreName, fieldName)
	}

	keywords := getGoKeywords()
	if keywords[name] {
		return fmt.Errorf("%w: %s", errGoKeyword, name)
	}

	first := rune(name[0])
	if !unicode.IsLetter(first) && first != '_' {
		return fmt.Errorf("%w: %s", errInvalidIdentifier, name)
	}

	for _, r := range name[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("%w: invalid character %q in %s", errInvalidStoreName, r, name)
		}
	}

	return nil
}

// getGoKeywords returns a map of Go keywords.
func getGoKeywords() map[string]bool {
	return map[string]bool{
		"break":       true,
		"case":        true,
		"chan":        true,
		"const":       true,
		"continue":    true,
		"default":     true,
		"defer":       true,
		"else":        true,
		"fallthrough": true,
		"for":         true,
		"func":        true,
		"go":          true,
		"goto":        true,
		"if":          true,
		"import":      true,
		"interface":   true,
		"map":         true,
		"package":     true,
		"range":       true,
		"return":      true,
		"select":      true,
		"struct":      true,
		"switch":      true,
		"type":        true,
		"var":         true,
	}
}

// validateStoreName validates store name before directory creation.
func validateStoreName(store *Info) error {
	if err := validateGoIdentifier(store.Name, "store name"); err != nil {
		return err
	}

	if store.Package == "" {
		return fmt.Errorf("%w", errEmptyPackageName)
	}

	if err := validateGoIdentifier(store.Package, "package name"); err != nil {
		return err
	}

	return nil
}

// generateSingleStore generates a single store.
func generateSingleStore(ctx *gofr.Context, cfg *Config, store *Info) error {
	if err := validateStoreName(store); err != nil {
		return fmt.Errorf("validation failed for store %q: %w", store.Name, err)
	}

	outputDir := store.OutputDir
	if outputDir == "" {
		outputDir = fmt.Sprintf("stores/%s", store.Name)
	}

	if err := os.MkdirAll(outputDir, defaultDirPerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Default implementation name if empty
	if store.Implementation == "" {
		store.Implementation = strings.ToLower(store.Name) + "Store"
	}

	storeConfig := &Config{
		Version: cfg.Version,
		Models:  cfg.Models,
		Stores:  []Info{*store},
	}

	if err := generateInterface(ctx, storeConfig, outputDir); err != nil {
		return fmt.Errorf("failed to generate interface: %w", err)
	}

	if err := generateImplementation(ctx, storeConfig, outputDir); err != nil {
		return fmt.Errorf("failed to generate implementation: %w", err)
	}

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

	var cfg Config

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		ctx.Logger.Errorf("Failed to parse config file: %v", err)
		return nil, errFailedToParseConfig
	}

	return &cfg, nil
}

// canonicalizeImport normalizes an import path for comparison.
func canonicalizeImport(imp string) string {
	imp = strings.TrimSpace(imp)
	parts := strings.Fields(imp)

	if len(parts) > 1 {
		imp = parts[len(parts)-1]
	}

	imp = strings.Trim(imp, `"`)
	imp = strings.TrimSpace(imp)

	return imp
}

// extractPackageName extracts the last component of an import path.
func extractPackageName(importPath string) string {
	canonical := canonicalizeImport(importPath)
	parts := strings.Split(canonical, "/")

	return parts[len(parts)-1]
}

// collectImports collects all required imports with canonicalization.
func collectImports(cfg *Config) ([]ImportInfo, ModelAliasMap) {
	imports := []ImportInfo{
		{Path: "gofr.dev/pkg/gofr", Alias: ""},
	}

	importMap := make(map[string]bool)
	pkgNameCount := make(map[string]int)
	modelAliasMap := make(ModelAliasMap)
	usedModels := getModelsUsedByStore(cfg)
	packageModels := make(map[string][]string)
	packageInfo := make(map[string]string)
	needsTime := checkNeedsTimeImport(cfg, usedModels)

	collectExternalModelImports(cfg, usedModels, importMap, packageInfo,
		packageModels, pkgNameCount)

	if needsTime && !importMap["time"] {
		imports = append(imports, ImportInfo{Path: "time", Alias: ""})
		importMap["time"] = true
	}

	usedAliases := make(map[string]bool)
	pathToAlias := make(map[string]string)

	for canonicalPath, pkgName := range packageInfo {
		alias := resolveAlias(pkgName, pkgNameCount, usedAliases, cfg)
		pathToAlias[canonicalPath] = alias
		imports = append(imports, ImportInfo{
			Path:  canonicalPath,
			Alias: alias,
		})
	}

	mapModelAliases(packageModels, pathToAlias, packageInfo, modelAliasMap)

	return imports, modelAliasMap
}

// checkNeedsTimeImport checks if time import is needed.
func checkNeedsTimeImport(cfg *Config, usedModels map[string]bool) bool {
	for i := range cfg.Models {
		model := &cfg.Models[i]
		if !usedModels[model.Name] {
			continue
		}

		for j := range model.Fields {
			if strings.Contains(model.Fields[j].Type, "time.Time") {
				return true
			}
		}
	}

	return false
}

// collectExternalModelImports collects external model imports.
func collectExternalModelImports(cfg *Config, usedModels map[string]bool,
	importMap map[string]bool, packageInfo map[string]string,
	packageModels map[string][]string, pkgNameCount map[string]int) {
	for i := range cfg.Models {
		model := &cfg.Models[i]
		if !usedModels[model.Name] {
			continue
		}

		if model.Path == "" || model.Package == "" {
			continue
		}

		canonicalPath := canonicalizeImport(model.Package)
		if !importMap[canonicalPath] {
			pkgName := extractPackageName(canonicalPath)
			packageInfo[canonicalPath] = pkgName
			importMap[canonicalPath] = true
			pkgNameCount[pkgName]++
		}

		packageModels[canonicalPath] = append(packageModels[canonicalPath], model.Name)
	}
}

// resolveAlias resolves the import alias.
func resolveAlias(pkgName string, pkgNameCount map[string]int,
	usedAliases map[string]bool, cfg *Config) string {
	if pkgNameCount[pkgName] <= 1 && pkgName != cfg.Stores[0].Package {
		return ""
	}

	alias := pkgName
	counter := 1

	for usedAliases[alias] {
		alias = fmt.Sprintf("%s%d", pkgName, counter)
		counter++
	}

	usedAliases[alias] = true

	return alias
}

// mapModelAliases maps model names to their aliases.
func mapModelAliases(packageModels map[string][]string, pathToAlias map[string]string,
	packageInfo map[string]string, modelAliasMap ModelAliasMap) {
	for canonicalPath, modelNames := range packageModels {
		aliasOrPkgName := pathToAlias[canonicalPath]
		if aliasOrPkgName == "" {
			aliasOrPkgName = packageInfo[canonicalPath]
		}

		for _, modelName := range modelNames {
			modelAliasMap[modelName] = aliasOrPkgName
		}
	}
}

// getModelsUsedByStore returns a map of model names used by the store.
func getModelsUsedByStore(cfg *Config) map[string]bool {
	usedModels := make(map[string]bool)

	if len(cfg.Stores) == 0 {
		return usedModels
	}

	for i := range cfg.Stores[0].Queries {
		query := &cfg.Stores[0].Queries[i]
		if query.Model != "" {
			usedModels[query.Model] = true
		}
	}

	return usedModels
}

// getModelTypeFn creates a function to get model types with proper package qualification.
func getModelTypeFn(cfg *Config, imports []ImportInfo, modelAliasMap ModelAliasMap) func(string) string {
	return func(modelName string) string {
		if alias, exists := modelAliasMap[modelName]; exists {
			return alias + "." + modelName
		}

		for i := range cfg.Models {
			model := &cfg.Models[i]
			if model.Name != modelName || model.Path == "" || model.Package == "" {
				continue
			}

			for j := range imports {
				imp := &imports[j]
				if canonicalizeImport(imp.Path) == canonicalizeImport(model.Package) {
					if imp.Alias != "" {
						return imp.Alias + "." + modelName
					}

					pkgName := extractPackageName(imp.Path)

					return pkgName + "." + modelName
				}
			}
		}

		return modelName
	}
}

// generateInterface generates the store interface file.
func generateInterface(ctx *gofr.Context, cfg *Config, outputDir string) error {
	interfaceFile := filepath.Join(outputDir, "interface.go")
	imports, modelAliasMap := collectImports(cfg)

	t, err := template.New("interface").Funcs(template.FuncMap{
		"getModelType": getModelTypeFn(cfg, imports, modelAliasMap),
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
		Imports []ImportInfo
	}{cfg.Stores[0], imports}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute interface template: %w", err)
	}

	ctx.Logger.Infof("Generated interface file: %s", interfaceFile)

	return nil
}

// generateImplementation generates the store implementation file.
func generateImplementation(ctx *gofr.Context, cfg *Config, outputDir string) error {
	implFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", cfg.Stores[0].Implementation))
	imports, modelAliasMap := collectImports(cfg)

	t, err := template.New("implementation").Funcs(template.FuncMap{
		"getModelType": getModelTypeFn(cfg, imports, modelAliasMap),
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
		Imports []ImportInfo
	}{cfg.Stores[0], imports}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute implementation template: %w", err)
	}

	ctx.Logger.Infof("Generated implementation file: %s", implFile)

	return nil
}

// generateModels generates model files or references existing ones.
func generateModels(ctx *gofr.Context, cfg *Config, outputDir string) error {
	usedModels := getModelsUsedByStore(cfg)

	for i := range cfg.Models {
		model := &cfg.Models[i]
		if !usedModels[model.Name] {
			continue
		}

		if model.Path != "" {
			ctx.Logger.Infof("Referencing existing model: %s from %s", model.Name, model.Path)
			continue
		}

		modelFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", strings.ToLower(model.Name)))
		store := cfg.Stores[0]

		if err := generateModelFile(ctx, modelFile, &store, model); err != nil {
			return err
		}
	}

	return nil
}

// generateModelFile generates a single model file.
func generateModelFile(ctx *gofr.Context, modelFile string, store *Info, model *Model) error {
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
	defer file.Close()

	if err := t.Execute(file, struct {
		Store Info
		Model Model
	}{*store, *model}); err != nil {
		return fmt.Errorf("failed to execute model template: %w", err)
	}

	ctx.Logger.Infof("Generated model file: %s", modelFile)

	return nil
}

// generateStoreConfig creates the initial store.yaml configuration file or appends to an existing one.
func generateStoreConfig(ctx *gofr.Context, storeName, storesDir string) error {
	configFile := filepath.Join(storesDir, "store.yaml")
	storeDir := fmt.Sprintf("stores/%s", strings.ToLower(storeName))

	// If file exists, append to it
	if _, err := os.Stat(configFile); err == nil {
		return appendToStoreConfig(ctx, configFile, storeName, storeDir)
	}

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
		StoreName          string
		PackageName        string
		OutputDir          string
		InterfaceName      string
		ImplementationName string
	}{
		StoreName:          storeName,
		PackageName:        strings.ToLower(storeName),
		OutputDir:          storeDir,
		InterfaceName:      cases.Title(language.English).String(storeName) + "Store",
		ImplementationName: strings.ToLower(storeName) + "Store",
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	ctx.Logger.Infof("Generated config file: %s", configFile)

	return nil
}

// appendToStoreConfig appends a new store to the existing store.yaml.
func appendToStoreConfig(ctx *gofr.Context, configFile, storeName, storeDir string) error {
	cfg, err := parseConfigFile(ctx, configFile)
	if err != nil {
		return err
	}

	// Check if store already exists
	for _, s := range cfg.Stores {
		if s.Name == storeName {
			return nil // Already exists
		}
	}

	newStore := Info{
		Name:           storeName,
		Package:        strings.ToLower(storeName),
		OutputDir:      storeDir,
		Interface:      cases.Title(language.English).String(storeName) + "Store",
		Implementation: strings.ToLower(storeName) + "Store",
	}

	cfg.Stores = append(cfg.Stores, newStore)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, defaultFilePerm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	ctx.Logger.Infof("Appended store %s to config file: %s", storeName, configFile)

	return nil
}

// generateInitialInterface creates the initial interface.go file.
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

// generateInitialStore creates the initial store.go file.
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
		ImplementationName: strings.ToLower(storeName) + "Store",
		InterfaceName:      cases.Title(language.English).String(storeName) + "Store",
	}

	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute store template: %w", err)
	}

	ctx.Logger.Infof("Generated store file: %s", storeFile)

	return nil
}

// appendStoreEntries appends new stores to stores/all.go.
func appendStoreEntries(ctx *gofr.Context, newStores []Entry) error {
	projectModule := detectProjectModule()
	if projectModule == "" {
		projectModule = "your-project"
	}

	content, err := os.ReadFile(allStoresFile)
	if err != nil {
		return generateCompleteAllFile(ctx, newStores, projectModule)
	}

	return processExistingAllFile(ctx, content, newStores, projectModule)
}

// processExistingAllFile processes the existing all.go file.
func processExistingAllFile(ctx *gofr.Context, content []byte,
	newStores []Entry, projectModule string) error {
	lines := strings.Split(string(content), "\n")
	existingStores, _ := parseExistingAllFile(lines)

	// Build the merged list: existing stores + genuinely new ones
	merged := make([]Entry, 0, len(existingStores)+len(newStores))

	for storeName := range existingStores {
		merged = append(merged, Entry{
			Name:          storeName,
			PackageName:   storeName,
			InterfaceName: cases.Title(language.English).String(storeName) + "Store",
		})
	}

	for i := range newStores {
		if !existingStores[newStores[i].Name] {
			merged = append(merged, newStores[i])
		}
	}

	if len(merged) == len(existingStores) {
		ctx.Logger.Info("All stores already exist in all.go")
		return nil
	}

	return generateCompleteAllFile(ctx, merged, projectModule)
}

// parseExistingAllFile parses the existing all.go file.
func parseExistingAllFile(lines []string) (existingStores, existingImports map[string]bool) {
	existingStores = make(map[string]bool)
	existingImports = make(map[string]bool)

	inImport := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "import (" {
			inImport = true
			continue
		}

		if inImport && trimmedLine == ")" {
			inImport = false
			continue
		}

		if inImport {
			existingImports[trimmedLine] = true
			continue
		}

		matches := storeRegex.FindStringSubmatch(line)
		if len(matches) >= minMatchLength {
			existingStores[matches[1]] = true
		}
	}

	return existingStores, existingImports
}

// generateCompleteAllFile generates a complete all.go file from scratch.

func generateCompleteAllFile(ctx *gofr.Context, stores []Entry, projectModule string) error {
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
