package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd"
	gofrConfig "gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

// createTestContext creates a test gofr.Context for CMD applications.
func createTestContext() *gofr.Context {
	c := container.NewContainer(gofrConfig.NewEnvFile("", logging.NewMockLogger(logging.DEBUG)))
	req := cmd.NewRequest([]string{})

	// Create context similar to examples/sample-cmd/main_test.go
	ctx := &gofr.Context{
		Context:   req.Context(),
		Request:   req,
		Container: c,
	}

	return ctx
}

func Test_validateGoIdentifier_Valid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
	}{
		{
			name:      "valid identifier starting with letter",
			input:     "user",
			fieldName: "store name",
		},
		{
			name:      "valid identifier starting with underscore",
			input:     "_user",
			fieldName: "store name",
		},
		{
			name:      "valid identifier with numbers",
			input:     "user123",
			fieldName: "store name",
		},
		{
			name:      "valid identifier with mixed case",
			input:     "UserStore",
			fieldName: "store name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGoIdentifier(tt.input, tt.fieldName)
			require.NoError(t, err)
		})
	}
}

func Test_validateGoIdentifier_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		errMsg    string
	}{
		{
			name:      "empty string",
			input:     "",
			fieldName: "store name",
			errMsg:    "store name cannot be empty",
		},
		{
			name:      "starts with number",
			input:     "123store",
			fieldName: "store name",
			errMsg:    "identifier must start with letter or underscore",
		},
		{
			name:      "contains hyphen",
			input:     "user-profile",
			fieldName: "store name",
			errMsg:    "invalid character",
		},
		{
			name:      "contains space",
			input:     "user profile",
			fieldName: "store name",
			errMsg:    "invalid character",
		},
		{
			name:      "Go keyword - if",
			input:     "if",
			fieldName: "store name",
			errMsg:    "cannot use Go keyword",
		},
		{
			name:      "Go keyword - func",
			input:     "func",
			fieldName: "store name",
			errMsg:    "cannot use Go keyword",
		},
		{
			name:      "Go keyword - package",
			input:     "package",
			fieldName: "package name",
			errMsg:    "cannot use Go keyword",
		},
		{
			name:      "contains special character",
			input:     "user@store",
			fieldName: "store name",
			errMsg:    "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGoIdentifier(tt.input, tt.fieldName)
			require.Error(t, err)

			if tt.errMsg != "" {
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func Test_validateStoreName(t *testing.T) {
	tests := []struct {
		name    string
		store   *Info
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid store name and package",
			store: &Info{
				Name:    "user",
				Package: "user",
			},
			wantErr: false,
		},
		{
			name: "invalid store name - starts with number",
			store: &Info{
				Name:    "123store",
				Package: "user",
			},
			wantErr: true,
			errMsg:  "identifier must start with letter or underscore",
		},
		{
			name: "invalid package name - contains hyphen",
			store: &Info{
				Name:    "user",
				Package: "user-profile",
			},
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name: "empty store name",
			store: &Info{
				Name:    "",
				Package: "user",
			},
			wantErr: true,
			errMsg:  "store name cannot be empty",
		},
		{
			name: "empty package name",
			store: &Info{
				Name:    "user",
				Package: "",
			},
			wantErr: true,
			errMsg:  "package name cannot be empty",
		},
		{
			name: "Go keyword as store name",
			store: &Info{
				Name:    "if",
				Package: "user",
			},
			wantErr: true,
			errMsg:  "cannot use Go keyword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStoreName(tt.store)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_canonicalizeImport(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple import",
			input:    `"gofr.dev/pkg/gofr"`,
			expected: "gofr.dev/pkg/gofr",
		},
		{
			name:     "import with alias",
			input:    `fmt "fmt"`,
			expected: "fmt",
		},
		{
			name:     "import with whitespace",
			input:    `  "gofr.dev/pkg/gofr"  `,
			expected: "gofr.dev/pkg/gofr",
		},
		{
			name:     "import with alias and whitespace",
			input:    `  fmt  "fmt"  `,
			expected: "fmt",
		},
		{
			name:     "quoted import",
			input:    `"time"`,
			expected: "time",
		},
		{
			name:     "unquoted import",
			input:    "time",
			expected: "time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canonicalizeImport(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_collectImports(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   []string
	}{
		{
			name: "basic config with no models",
			config: &Config{
				Stores: []Info{
					{
						Name:    "user",
						Package: "user",
						Queries: []Query{
							{
								Name: "GetByID",
								Type: "select",
							},
						},
					},
				},
			},
			want: []string{"gofr.dev/pkg/gofr"},
		},
		{
			name: "config with time model",
			config: &Config{
				Models: []Model{
					{
						Name: "User",
						Fields: []Field{
							{Name: "CreatedAt", Type: "time.Time"},
						},
					},
				},
				Stores: []Info{
					{
						Name:    "user",
						Package: "user",
						Queries: []Query{
							{
								Name:  "GetByID",
								Type:  "select",
								Model: "User",
							},
						},
					},
				},
			},
			want: []string{"gofr.dev/pkg/gofr", "time"},
		},
		{
			name: "config with multiple stores",
			config: &Config{
				Stores: []Info{
					{Name: "user", Package: "user"},
					{Name: "product", Package: "product"},
				},
			},
			want: []string{"gofr.dev/pkg/gofr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imports, _ := collectImports(tt.config)
			// Check that all expected imports are present
			for _, want := range tt.want {
				found := false

				for _, imp := range imports {
					if canonicalizeImport(imp.Path) == canonicalizeImport(want) {
						found = true

						break
					}
				}

				assert.True(t, found, "expected import %q not found", want)
			}
		})
	}
}

func Test_generateModelFile(t *testing.T) {
	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "user.go")

	store := Info{
		Name:    "user",
		Package: "user",
	}

	model := Model{
		Name: "User",
		Fields: []Field{
			{Name: "ID", Type: "int64", Tag: `db:"id" json:"id"`},
			{Name: "Name", Type: "string", Tag: `db:"name" json:"name"`},
		},
	}

	ctx := createTestContext()
	err := generateModelFile(ctx, modelFile, &store, &model)
	require.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(modelFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, contentStr, "package user")
	assert.Contains(t, contentStr, "type User struct")
	assert.Contains(t, contentStr, "ID int64")
	assert.Contains(t, contentStr, "Name string")
	assert.Contains(t, contentStr, "TableName()")
}

func Test_generateSingleStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := createTestContext()

	cfg := &Config{
		Version: "1.0",
		Models: []Model{
			{
				Name: "User",
				Fields: []Field{
					{Name: "ID", Type: "int64", Tag: `db:"id"`},
					{Name: "Name", Type: "string", Tag: `db:"name"`},
				},
			},
		},
		Stores: []Info{
			{
				Name:           "user",
				Package:        "user",
				OutputDir:      filepath.Join(tmpDir, "stores", "user"),
				Interface:      "User",
				Implementation: "userStore",
				Queries: []Query{
					{
						Name:    "GetByID",
						SQL:     "SELECT * FROM users WHERE id = ?",
						Type:    "select",
						Model:   "User",
						Returns: "single",
						Params: []QueryParam{
							{Name: "id", Type: "int64"},
						},
					},
				},
			},
		},
	}

	store := &cfg.Stores[0]
	err := generateSingleStore(ctx, cfg, store)
	require.NoError(t, err)

	// Verify files were created
	interfaceFile := filepath.Join(store.OutputDir, "interface.go")
	// Implementation file name is based on Implementation field, not "store.go"
	implFile := filepath.Join(store.OutputDir, fmt.Sprintf("%s.go", store.Implementation))
	modelFile := filepath.Join(store.OutputDir, "user.go")

	assert.FileExists(t, interfaceFile)
	assert.FileExists(t, implFile)
	assert.FileExists(t, modelFile)

	// Verify interface content
	interfaceContent, err := os.ReadFile(interfaceFile)
	require.NoError(t, err)

	interfaceStr := string(interfaceContent)
	assert.Contains(t, interfaceStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, interfaceStr, "type User interface")
	assert.Contains(t, interfaceStr, "GetByID(ctx *gofr.Context, id int64)")

	// Verify implementation content
	implContent, err := os.ReadFile(implFile)
	require.NoError(t, err)

	implStr := string(implContent)
	assert.Contains(t, implStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, implStr, "type userStore struct")
	assert.Contains(t, implStr, "func NewUser() User")
}

func Test_generateSingleStore_InvalidName(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := createTestContext()

	cfg := &Config{
		Stores: []Info{
			{
				Name:           "123store", // Invalid: starts with number
				Package:        "user",
				OutputDir:      filepath.Join(tmpDir, "stores", "user"),
				Interface:      "User",
				Implementation: "userStore",
			},
		},
	}

	store := &cfg.Stores[0]
	err := generateSingleStore(ctx, cfg, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "identifier must start with letter or underscore")
}

func Test_detectProjectModule(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Test without go.mod
	module := detectProjectModule()
	assert.Empty(t, module)

	// Create go.mod
	goModContent := `module test-project

go 1.22
`
	err := os.WriteFile("go.mod", []byte(goModContent), 0600)
	require.NoError(t, err)

	module = detectProjectModule()
	assert.Equal(t, "test-project", module)
}

func Test_parseExistingAllFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantStores  map[string]Entry
		wantImports []string
	}{
		{
			name: "valid all.go with stores",
			content: `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package stores

import (
	"test-project/stores/user"
	"test-project/stores/product"
)

func All() map[string]func() any {
	return map[string]func() any {
		"user": func() any {
			return user.NewUser()
		},
		"product": func() any {
			return product.NewProduct()
		},
	}
}
`,
			wantStores: map[string]Entry{
				"user": {
					Name:          "user",
					PackageName:   "user",
					InterfaceName: "User",
				},
				"product": {
					Name:          "product",
					PackageName:   "product",
					InterfaceName: "Product",
				},
			},
			wantImports: []string{
				"test-project/stores/user",
				"test-project/stores/product",
			},
		},
		{
			name: "empty all.go",
			content: `package stores

func All() map[string]func() any {
	return map[string]func() any {
	}
}
`,
			wantStores:  map[string]Entry{},
			wantImports: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.content, "\n")
			stores, imports := parseExistingAllFile(lines)

			// parseExistingAllFile returns map[string]bool, so we just check existence
			assert.Len(t, stores, len(tt.wantStores))

			for name := range tt.wantStores {
				assert.True(t, stores[name], "store %q not found", name)
			}

			// Check imports (order may vary)
			assert.Len(t, imports, len(tt.wantImports))

			for _, wantImp := range tt.wantImports {
				found := false

				for imp := range imports {
					if canonicalizeImport(imp) == canonicalizeImport(wantImp) {
						found = true

						break
					}
				}

				assert.True(t, found, "import %q not found", wantImp)
			}
		})
	}
}

func Test_parseExistingAllFileString_Fallback(t *testing.T) {
	// Test fallback string parser with malformed content
	content := `package stores

func All() map[string]func() any {
	return map[string]func() any {
		"user": func() any {
			return user.NewUser()
		},
	}
}
`

	lines := strings.Split(content, "\n")
	stores, _ := parseExistingAllFile(lines)

	// Should at least find the store name
	assert.NotEmpty(t, stores)
	assert.True(t, stores["user"], "store 'user' should be found")
}

func Test_appendStoreEntries_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create go.mod
	err := os.WriteFile("go.mod", []byte("module test-project\n"), 0600)
	require.NoError(t, err)

	ctx := createTestContext()

	newStores := []Entry{
		{
			Name:          "user",
			PackageName:   "user",
			InterfaceName: "UserStore",
		},
	}

	err = appendStoreEntries(ctx, newStores)
	require.NoError(t, err)

	// Verify all.go was created
	allFilePath := filepath.Join("stores", "all.go")
	assert.FileExists(t, allFilePath)

	content, err := os.ReadFile(allFilePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, contentStr, `"user": func() any`)
	assert.Contains(t, contentStr, "user.NewUserStore()")
}

func Test_appendStoreEntries_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create go.mod
	err := os.WriteFile("go.mod", []byte("module test-project\n"), 0600)
	require.NoError(t, err)

	// Create stores directory and all.go
	err = os.MkdirAll("stores", 0755)
	require.NoError(t, err)

	existingContent := `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package stores

import (
	"test-project/stores/user"
)

func All() map[string]func() any {
	return map[string]func() any {
		"user": func() any {
			return user.NewUser()
		},
	}
}
`
	err = os.WriteFile("stores/all.go", []byte(existingContent), 0600)
	require.NoError(t, err)

	ctx := createTestContext()

	// Try to add user again (should be skipped) and product (should be added)
	newStores := []Entry{
		{
			Name:          "user",
			PackageName:   "user",
			InterfaceName: "UserStore",
		},
		{
			Name:          "product",
			PackageName:   "product",
			InterfaceName: "ProductStore",
		},
	}

	err = appendStoreEntries(ctx, newStores)
	require.NoError(t, err)

	// Verify all.go was updated
	content, err := os.ReadFile("stores/all.go")
	require.NoError(t, err)

	contentStr := string(content)
	// Should still have user (already existed, regenerated with correct naming)
	assert.Contains(t, contentStr, `"user": func() any`)
	assert.Contains(t, contentStr, "user.NewUserStore()")
	// Should have product added (new)
	assert.Contains(t, contentStr, `"product": func() any`)
	assert.Contains(t, contentStr, "product.NewProductStore()")
	// Should have both imports
	assert.Contains(t, contentStr, `"test-project/stores/user"`)
	assert.Contains(t, contentStr, `"test-project/stores/product"`)
}

func Test_storeRegex(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantMatch bool
		wantName  string
	}{
		{
			name:      "standard format",
			line:      `        "user": func() any {`,
			wantMatch: true,
			wantName:  "user",
		},
		{
			name:      "with extra whitespace",
			line:      `   "product"   :   func ( ) any {`,
			wantMatch: true,
			wantName:  "product",
		},
		{
			name:      "no whitespace",
			line:      `"order":func()any{`,
			wantMatch: true,
			wantName:  "order",
		},
		{
			name:      "not a store entry",
			line:      `        "user": "value",`,
			wantMatch: false,
		},
		{
			name:      "different function signature",
			line:      `        "user": func() string {`,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := storeRegex.FindStringSubmatch(tt.line)
			if tt.wantMatch {
				require.GreaterOrEqual(t, len(matches), 2, "expected match")
				assert.Equal(t, tt.wantName, matches[1])
			} else {
				assert.Empty(t, matches, "should not match")
			}
		})
	}
}

func Test_generateModels(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := createTestContext()

	cfg := &Config{
		Stores: []Info{
			{
				Name:    "user",
				Package: "user",
				Queries: []Query{
					{Model: "User"},    // Used
					{Model: "Product"}, // Used
				},
			},
		},
		Models: []Model{
			{
				Name: "User",
				Fields: []Field{
					{Name: "ID", Type: "int64", Tag: `db:"id"`},
					{Name: "Name", Type: "string", Tag: `db:"name"`},
				},
			},
			{
				Name: "Product",
				Fields: []Field{
					{Name: "ID", Type: "int64", Tag: `db:"id"`},
				},
			},
		},
	}

	outputDir := filepath.Join(tmpDir, "stores", "user")
	err := os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	err = generateModels(ctx, cfg, outputDir)
	require.NoError(t, err)

	// Verify model files were created
	userModelFile := filepath.Join(outputDir, "user.go")
	productModelFile := filepath.Join(outputDir, "product.go")

	assert.FileExists(t, userModelFile)
	assert.FileExists(t, productModelFile)

	// Verify content
	userContent, err := os.ReadFile(userModelFile)
	require.NoError(t, err)
	assert.Contains(t, string(userContent), "type User struct")
}

func Test_generateModels_UnusedModel(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := createTestContext()

	cfg := &Config{
		Stores: []Info{
			{
				Name:    "user",
				Package: "user",
				Queries: []Query{
					{Model: "User"}, // Only User is used
				},
			},
		},
		Models: []Model{
			{
				Name: "User",
				Fields: []Field{
					{Name: "ID", Type: "int64", Tag: `db:"id"`},
				},
			},
			{
				Name: "Unused",
				Fields: []Field{
					{Name: "ID", Type: "int64", Tag: `db:"id"`},
				},
			},
		},
	}

	outputDir := filepath.Join(tmpDir, "stores", "user")
	err := os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	err = generateModels(ctx, cfg, outputDir)
	require.NoError(t, err)

	// Only User model should be created
	userModelFile := filepath.Join(outputDir, "user.go")
	unusedModelFile := filepath.Join(outputDir, "unused.go")

	assert.FileExists(t, userModelFile)
	assert.NoFileExists(t, unusedModelFile)
}

func Test_generateInterface(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := createTestContext()

	store := &Info{
		Name:      "user",
		Package:   "user",
		Interface: "User",
		Queries: []Query{
			{
				Name:    "GetByID",
				Type:    "select",
				Model:   "User",
				Returns: "single",
				Params: []QueryParam{
					{Name: "id", Type: "int64"},
				},
			},
		},
	}

	outputDir := filepath.Join(tmpDir, "stores", "user")
	err := os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	// Create a config with the store
	testConfig := &Config{
		Stores: []Info{*store},
	}
	err = generateInterface(ctx, testConfig, outputDir)
	require.NoError(t, err)

	interfaceFile := filepath.Join(outputDir, "interface.go")
	assert.FileExists(t, interfaceFile)

	content, err := os.ReadFile(interfaceFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, contentStr, "package user")
	assert.Contains(t, contentStr, "type User interface")
	assert.Contains(t, contentStr, "GetByID(ctx *gofr.Context, id int64)")
}

func Test_generateImplementation(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := createTestContext()

	store := &Info{
		Name:           "user",
		Package:        "user",
		Interface:      "User",
		Implementation: "userStore",
		Queries: []Query{
			{
				Name:    "GetByID",
				SQL:     "SELECT * FROM users WHERE id = ?",
				Type:    "select",
				Model:   "User",
				Returns: "single",
				Params: []QueryParam{
					{Name: "id", Type: "int64"},
				},
			},
		},
	}

	outputDir := filepath.Join(tmpDir, "stores", "user")
	err := os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	// Create a config with the store
	testConfig := &Config{
		Stores: []Info{*store},
	}
	err = generateImplementation(ctx, testConfig, outputDir)
	require.NoError(t, err)

	implFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", store.Implementation))
	assert.FileExists(t, implFile)

	content, err := os.ReadFile(implFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, contentStr, "package user")
	assert.Contains(t, contentStr, "type userStore struct")
	assert.Contains(t, contentStr, "func NewUser() User")
	assert.Contains(t, contentStr, "func (s *userStore) GetByID")
}

func Test_processExistingAllFile_AllStoresExist(t *testing.T) {
	ctx := createTestContext()

	existingContent := `package stores

func All() map[string]func() any {
	return map[string]func() any {
		"user": func() any {
			return user.NewUser()
		},
	}
}
`

	lines := strings.Split(existingContent, "\n")
	_, _ = parseExistingAllFile(lines)

	newStores := []Entry{
		{
			Name:          "user",
			PackageName:   "user",
			InterfaceName: "User",
		},
	}

	err := processExistingAllFile(ctx, []byte(existingContent), newStores, "test-project")
	require.NoError(t, err)
}

func Test_splitContentToLines(t *testing.T) {
	content := []byte("line1\nline2\nline3")
	lines := strings.Split(string(content), "\n")

	assert.Len(t, lines, 3)
	assert.Equal(t, "line1", lines[0])
	assert.Equal(t, "line2", lines[1])
	assert.Equal(t, "line3", lines[2])
}

func Test_createNewAllFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := createTestContext()

	stores := []Entry{
		{
			Name:          "user",
			PackageName:   "user",
			InterfaceName: "User",
		},
		{
			Name:          "product",
			PackageName:   "product",
			InterfaceName: "Product",
		},
	}

	err := generateCompleteAllFile(ctx, stores, "test-project")
	require.NoError(t, err)

	allFilePath := filepath.Join("stores", "all.go")
	assert.FileExists(t, allFilePath)

	content, err := os.ReadFile(allFilePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "Code generated by gofr.dev/cli/gofr. DO NOT EDIT.")
	assert.Contains(t, contentStr, "package stores")
	assert.Contains(t, contentStr, `"user": func() any`)
	assert.Contains(t, contentStr, `"product": func() any`)
	assert.Contains(t, contentStr, "user.NewUser()")
	assert.Contains(t, contentStr, "product.NewProduct()")
}

func Test_InitStore(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	c := container.NewContainer(gofrConfig.NewEnvFile("", logging.NewMockLogger(logging.DEBUG)))
	req := cmd.NewRequest([]string{"executable", "store", "init", "-name=user"})

	ctx := &gofr.Context{
		Context:   req.Context(),
		Request:   req,
		Container: c,
	}

	_, err := InitStore(ctx)
	require.NoError(t, err)

	// Verify stores directory structure
	assert.DirExists(t, "stores")
	assert.DirExists(t, filepath.Join("stores", "user"))

	// Verify store.yaml is in stores/ directory
	assert.FileExists(t, filepath.Join("stores", "store.yaml"))

	// Verify store.yaml content
	content, err := os.ReadFile(filepath.Join("stores", "store.yaml"))
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "user")
	assert.Contains(t, contentStr, "stores/user")

	// Verify initial interface and implementation are in stores/user/
	assert.FileExists(t, filepath.Join("stores", "user", "interface.go"))
	assert.FileExists(t, filepath.Join("stores", "user", "user.go"))
}

func Test_InitStore_Append(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	c := container.NewContainer(gofrConfig.NewEnvFile("", logging.NewMockLogger(logging.DEBUG)))

	// Initial store
	req1 := cmd.NewRequest([]string{"executable", "store", "init", "-name=user"})
	ctx1 := &gofr.Context{Context: req1.Context(), Request: req1, Container: c}
	_, err := InitStore(ctx1)
	require.NoError(t, err)

	// Append second store
	req2 := cmd.NewRequest([]string{"executable", "store", "init", "-name=product"})
	ctx2 := &gofr.Context{Context: req2.Context(), Request: req2, Container: c}
	_, err = InitStore(ctx2)
	require.NoError(t, err)

	// Verify both exist in store.yaml
	content, err := os.ReadFile(filepath.Join("stores", "store.yaml"))
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "name: user")
	assert.Contains(t, contentStr, "name: product")
}

func Test_InitStore_InvalidName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	c := container.NewContainer(gofrConfig.NewEnvFile("", logging.NewMockLogger(logging.DEBUG)))
	req := cmd.NewRequest([]string{"executable", "store", "init", "-name=123store"})
	ctx := &gofr.Context{Context: req.Context(), Request: req, Container: c}

	_, err := InitStore(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid store name")

	// Verify no store directory was created
	assert.NoDirExists(t, filepath.Join("stores", "123store"))
}

type generateStoreTestCase struct {
	name        string
	yamlContent string
	configFlag  string
	wantErr     bool
	errContains string
	verify      func(t *testing.T, dir string)
}

const (
	yamlSingleStoreSelectSingle = `version: "1.0"
models:
  - name: "User"
    fields:
      - name: "ID"
        type: "int64"
        tag: 'db:"id" json:"id"'
      - name: "Name"
        type: "string"
        tag: 'db:"name" json:"name"'
stores:
  - name: "user"
    package: "user"
    output_dir: "stores/user"
    interface: "UserStore"
    implementation: "userStore"
    queries:
      - name: "GetUserByID"
        sql: "SELECT id, name FROM users WHERE id = ?"
        type: "select"
        model: "User"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
`

	yamlSingleStoreMultipleReturnTypes = `version: "1.0"
models:
  - name: "Product"
    fields:
      - name: "ID"
        type: "int64"
        tag: 'db:"id"'
      - name: "Name"
        type: "string"
        tag: 'db:"name"'
stores:
  - name: "product"
    package: "product"
    output_dir: "stores/product"
    interface: "ProductStore"
    implementation: "productStore"
    queries:
      - name: "GetByID"
        sql: "SELECT * FROM products WHERE id = ?"
        type: "select"
        model: "Product"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
      - name: "ListAll"
        sql: "SELECT * FROM products"
        type: "select"
        model: "Product"
        returns: "multiple"
      - name: "CountActive"
        sql: "SELECT COUNT(*) FROM products WHERE active = 1"
        type: "select"
        model: "Product"
        returns: "count"
      - name: "CreateProduct"
        sql: "INSERT INTO products (name) VALUES (?)"
        type: "insert"
        model: "Product"
        returns: "custom"
        params:
          - name: "name"
            type: "string"
`

	yamlMultipleStores = `version: "1.0"
models:
  - name: "User"
    fields:
      - name: "ID"
        type: "int64"
        tag: 'db:"id"'
  - name: "Order"
    fields:
      - name: "ID"
        type: "int64"
        tag: 'db:"id"'
stores:
  - name: "user"
    package: "user"
    output_dir: "stores/user"
    interface: "UserStore"
    implementation: "userStore"
    queries:
      - name: "GetByID"
        sql: "SELECT id FROM users WHERE id = ?"
        type: "select"
        model: "User"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
  - name: "order"
    package: "order"
    output_dir: "stores/order"
    interface: "OrderStore"
    implementation: "orderStore"
    queries:
      - name: "GetByID"
        sql: "SELECT id FROM orders WHERE id = ?"
        type: "select"
        model: "Order"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
`

	yamlEmptyQueries = `version: "1.0"
stores:
  - name: "empty"
    package: "empty"
    output_dir: "stores/empty"
    interface: "EmptyStore"
    implementation: "emptyStore"
    queries: []
`

	yamlCustomConfigStore = `version: "1.0"
stores:
  - name: "custom"
    package: "custom"
    output_dir: "stores/custom"
    interface: "CustomStore"
    implementation: "customStore"
    queries:
      - name: "GetAll"
        sql: "SELECT * FROM custom"
        type: "select"
        model: "Custom"
        returns: "multiple"
`

	yamlExternalModelReference = `version: "1.0"
models:
  - name: "User"
    path: "../models/user.go"
    package: "test-project/models"
stores:
  - name: "user"
    package: "user"
    output_dir: "stores/user"
    interface: "UserStore"
    implementation: "userStore"
    queries:
      - name: "GetByID"
        sql: "SELECT id FROM users WHERE id = ?"
        type: "select"
        model: "User"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
`
)

func generateStoreTestCases() []generateStoreTestCase {
	return []generateStoreTestCase{
		{
			name:        "single store - select single return",
			yamlContent: yamlSingleStoreSelectSingle,
			verify:      verifySingleStoreSelectSingle,
		},
		{
			name:        "single store - multiple return types",
			yamlContent: yamlSingleStoreMultipleReturnTypes,
			verify:      verifySingleStoreMultipleReturnTypes,
		},
		{
			name:        "multiple stores in one YAML",
			yamlContent: yamlMultipleStores,
			verify:      verifyMultipleStores,
		},
		{
			name:        "empty queries list",
			yamlContent: yamlEmptyQueries,
			verify:      verifyEmptyQueries,
		},
		{
			name:        "custom config path via -config flag",
			yamlContent: yamlCustomConfigStore,
			configFlag:  "custom/myconfig.yaml",
			verify: func(t *testing.T, dir string) {
				t.Helper()
				assert.FileExists(t, filepath.Join(dir, "stores", "custom", "interface.go"))
				assert.FileExists(t, filepath.Join(dir, "stores", "custom", "customStore.go"))
			},
		},
		{
			name:        "config file not found",
			configFlag:  "nonexistent/config.yaml",
			wantErr:     true,
			errContains: "error opening the config file",
		},
		{
			name:        "no stores defined in YAML",
			yamlContent: "version: \"1.0\"\nstores: []\n",
			wantErr:     true,
			errContains: "no stores defined in configuration",
		},
		{
			name:        "store with external model reference",
			yamlContent: yamlExternalModelReference,
			verify:      verifyExternalModelReference,
		},
	}
}

func verifySingleStoreSelectSingle(t *testing.T, dir string) {
	t.Helper()

	assert.DirExists(t, filepath.Join(dir, "stores", "user"))
	assert.FileExists(t, filepath.Join(dir, "stores", "all.go"))
	assert.FileExists(t, filepath.Join(dir, "stores", "user", "interface.go"))
	assert.FileExists(t, filepath.Join(dir, "stores", "user", "userStore.go"))
	assert.FileExists(t, filepath.Join(dir, "stores", "user", "user.go"))

	ifaceContent, err := os.ReadFile(filepath.Join(dir, "stores", "user", "interface.go"))
	require.NoError(t, err)

	ifaceStr := string(ifaceContent)
	assert.Contains(t, ifaceStr, "type UserStore interface")
	assert.Contains(t, ifaceStr, "GetUserByID(ctx *gofr.Context, id int64)")
	assert.Contains(t, ifaceStr, "(User, error)")

	implContent, err := os.ReadFile(filepath.Join(dir, "stores", "user", "userStore.go"))
	require.NoError(t, err)

	implStr := string(implContent)
	assert.Contains(t, implStr, "func NewUserStore() UserStore")
	assert.Contains(t, implStr, "TODO: Implement GetUserByID query")

	modelContent, err := os.ReadFile(filepath.Join(dir, "stores", "user", "user.go"))
	require.NoError(t, err)

	modelStr := string(modelContent)
	assert.Contains(t, modelStr, "type User struct")
	assert.Contains(t, modelStr, "TableName()")

	allContent, err := os.ReadFile(filepath.Join(dir, "stores", "all.go"))
	require.NoError(t, err)

	allStr := string(allContent)
	assert.Contains(t, allStr, `"user": func() any`)
	assert.Contains(t, allStr, "user.NewUserStore()")
}

func verifySingleStoreMultipleReturnTypes(t *testing.T, dir string) {
	t.Helper()

	ifaceContent, err := os.ReadFile(filepath.Join(dir, "stores", "product", "interface.go"))
	require.NoError(t, err)

	ifaceStr := string(ifaceContent)

	assert.Contains(t, ifaceStr, "GetByID(ctx *gofr.Context, id int64)")
	assert.Contains(t, ifaceStr, "(Product, error)")
	assert.Contains(t, ifaceStr, "ListAll(ctx *gofr.Context)")
	assert.Contains(t, ifaceStr, "([]Product, error)")
	assert.Contains(t, ifaceStr, "CountActive(ctx *gofr.Context)")
	assert.Contains(t, ifaceStr, "(int64, error)")
	assert.Contains(t, ifaceStr, "CreateProduct(ctx *gofr.Context, name string)")
	assert.Contains(t, ifaceStr, "(any, error)")
}

func verifyMultipleStores(t *testing.T, dir string) {
	t.Helper()

	assert.DirExists(t, filepath.Join(dir, "stores", "user"))
	assert.DirExists(t, filepath.Join(dir, "stores", "order"))
	assert.FileExists(t, filepath.Join(dir, "stores", "user", "interface.go"))
	assert.FileExists(t, filepath.Join(dir, "stores", "order", "interface.go"))

	allContent, err := os.ReadFile(filepath.Join(dir, "stores", "all.go"))
	require.NoError(t, err)

	allStr := string(allContent)
	assert.Contains(t, allStr, `"user": func() any`)
	assert.Contains(t, allStr, "user.NewUserStore()")
	assert.Contains(t, allStr, `"order": func() any`)
	assert.Contains(t, allStr, "order.NewOrderStore()")
}

func verifyEmptyQueries(t *testing.T, dir string) {
	t.Helper()

	assert.FileExists(t, filepath.Join(dir, "stores", "empty", "interface.go"))
	assert.FileExists(t, filepath.Join(dir, "stores", "empty", "emptyStore.go"))

	ifaceContent, err := os.ReadFile(filepath.Join(dir, "stores", "empty", "interface.go"))
	require.NoError(t, err)
	assert.Contains(t, string(ifaceContent), "type EmptyStore interface")
}

func verifyExternalModelReference(t *testing.T, dir string) {
	t.Helper()

	assert.FileExists(t, filepath.Join(dir, "stores", "user", "interface.go"))
	assert.NoFileExists(t, filepath.Join(dir, "stores", "user", "user.go"))

	ifaceContent, err := os.ReadFile(filepath.Join(dir, "stores", "user", "interface.go"))
	require.NoError(t, err)
	assert.Contains(t, string(ifaceContent), "test-project/models")
}

func runGenerateStoreTest(t *testing.T, tt *generateStoreTestCase) {
	t.Helper()

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := os.WriteFile("go.mod", []byte("module test-project\n\ngo 1.22\n"), 0600)
	require.NoError(t, err)

	configPath := filepath.Join("stores", "store.yaml")
	args := []string{}

	if tt.configFlag != "" {
		configPath = tt.configFlag
		args = append(args, "-config="+tt.configFlag)
	}

	if tt.yamlContent != "" {
		dir := filepath.Dir(configPath)
		if dir != "." {
			err = os.MkdirAll(dir, 0755)
			require.NoError(t, err)
		} else {
			err = os.MkdirAll("stores", 0755)
			require.NoError(t, err)
		}

		err = os.WriteFile(configPath, []byte(tt.yamlContent), 0600)
		require.NoError(t, err)
	}

	c := container.NewContainer(gofrConfig.NewEnvFile("", logging.NewMockLogger(logging.DEBUG)))
	req := cmd.NewRequest(args)
	ctx := &gofr.Context{Context: req.Context(), Request: req, Container: c}

	_, err = GenerateStore(ctx)

	if tt.wantErr {
		require.Error(t, err)

		if tt.errContains != "" {
			assert.Contains(t, err.Error(), tt.errContains)
		}

		return
	}

	require.NoError(t, err)

	if tt.verify != nil {
		tt.verify(t, tmpDir)
	}
}

func Test_GenerateStore(t *testing.T) {
	for _, tt := range generateStoreTestCases() {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			runGenerateStoreTest(t, &tc)
		})
	}
}
