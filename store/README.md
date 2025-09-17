# GoFr Store Generator

The GoFr Store Generator is a CLI tool that generates store layer functions with context support using YAML configuration files. It provides robust code generation with proper appending functionality, mixed model support, and comprehensive linter compliance.

## Features

- **YAML Configuration**: Define your models and queries in a simple YAML file
- **Context Support**: All generated methods include `*gofr.Context` as the first parameter
- **Store Struct Pattern**: Follows GoFr's store layer structure with methods on store structs
- **Multiple Query Types**: Supports select, insert, update, delete, transaction, and health operations
- **Flexible Return Types**: Supports single, multiple, count, and custom return types
- **Model Generation**: Automatically generates Go structs for your data models
- **External Model Support**: Reference existing model files instead of generating new ones
- **Multi-Store Support**: Generate multiple stores from a single YAML configuration
- **Store Isolation**: Each store only generates files relevant to its own models and queries
- **Mixed Model Support**: Combine external and generated models in the same project
- **Smart Import Management**: Only imports packages that are actually used by each store
- **Store Registry with Appending**: Properly appends new stores to `stores/all.go` without overwriting existing entries
- **Duplicate Prevention**: Prevents duplicate stores and imports in the registry
- **GoFr Integration**: Uses `ctx.SQL()` for database operations with proper GoFr patterns
- **Code Compilation**: Generated code compiles successfully and follows Go best practices
- **Linter Compliance**: All generated code passes strict linter checks including `golangci-lint`

## Commands

### Initialize Store

Create a new store with initial configuration and directory structure:

```bash
gofr store init -name=<store_name>
```

This command:
- Creates the `stores/<store_name>` directory
- Generates a `store.yaml` configuration file with examples
- Creates initial `interface.go` and `<store_name>.go` files
- Sets up the basic store structure
- Updates or creates `stores/all.go` registry with the new store (appending mode)

### Generate Store Code

Generate store layer code from YAML configuration:

```bash
gofr store generate -config=<path_to_config>
```

If no config path is specified, it defaults to `stores/store.yaml`.

## Usage Examples

```bash
# Initialize a new user store
gofr store init -name=user

# Initialize a product store (appends to existing all.go)
gofr store init -name=product

# Generate store code from configuration
gofr store generate -config=stores/user/store.yaml

# Generate multiple stores from a single configuration
gofr store generate -config=multi-store.yaml

# Generate with default config path
gofr store generate
```

## Integration with GoFr Application

### Setting Up Store Generator in main.go

To integrate the store generator commands with your GoFr application, add the following to your `main.go`:

```go
package main

import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/cmd"
    
    // Import your store generator package
    "your-project/store"  // Update this to match your project structure
)

func main() {
    // Initialize the GoFr application
    app := gofr.New()

    // Register store generator commands
    app.SubCommand("store", func(c *cmd.Context) (interface{}, error) {
        subCmd := c.Param("subcommand")
        
        switch subCmd {
        case "init":
            return store.InitStore(c.Context)
        case "generate":
            return store.GenerateStore(c.Context)
        default:
            return nil, fmt.Errorf("unknown store subcommand: %s. Available: init, generate", subCmd)
        }
    })

    // Your other application routes and logic
    app.GET("/health", func(ctx *gofr.Context) (interface{}, error) {
        return "OK", nil
    })

    // Start the application
    app.Start()
}
```

### Alternative: Separate CLI Tool

For a dedicated CLI tool approach, create a separate `cli/main.go`:

```go
package main

import (
    "fmt"
    "os"
    
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/cmd"
    
    // Import your store generator package
    "your-project/store"
)

func main() {
    // Initialize GoFr for CLI usage
    app := gofr.New()

    // Handle store commands
    args := os.Args
    if len(args) < 2 {
        fmt.Println("Usage: cli store <command> [options]")
        fmt.Println("Commands:")
        fmt.Println("  init -name=<store_name>")
        fmt.Println("  generate -config=<config_file>")
        os.Exit(1)
    }

    if args[1] == "store" {
        if len(args) < 3 {
            fmt.Println("Please specify a store command: init or generate")
            os.Exit(1)
        }

        // Create a context with command line arguments
        ctx := &gofr.Context{
            // Set up context with CLI parameters
        }

        // Parse command line arguments
        params := make(map[string]string)
        for _, arg := range args[3:] {
            if strings.HasPrefix(arg, "-") {
                parts := strings.SplitN(arg[1:], "=", 2)
                if len(parts) == 2 {
                    params[parts[0]] = parts[1]
                }
            }
        }

        // Simulate GoFr context for CLI usage
        ctx.Request = &http.Request{
            URL: &url.URL{
                RawQuery: buildQueryString(params),
            },
        }

        var result interface{}
        var err error

        switch args[2] {
        case "init":
            result, err = store.InitStore(ctx)
        case "generate":
            result, err = store.GenerateStore(ctx)
        default:
            fmt.Printf("Unknown command: %s\n", args[2])
            os.Exit(1)
        }

        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }

        fmt.Println(result)
    }
}

func buildQueryString(params map[string]string) string {
    var query []string
    for key, value := range params {
        query = append(query, fmt.Sprintf("%s=%s", key, value))
    }
    return strings.Join(query, "&")
}
```

### Using Generated Stores in Your Application

Once you've generated your stores, integrate them into your GoFr application:

```go
package main

import (
    "gofr.dev/pkg/gofr"
    
    // Import generated stores
    "your-project/stores"
    "your-project/stores/user"
    "your-project/stores/product"
)

func main() {
    app := gofr.New()

    // Initialize stores using the generated registry
    allStores := stores.All()
    
    // Get specific stores
    userStore := stores.GetStore("user").(user.UserStore)
    productStore := stores.GetStore("product").(product.ProductStore)

    // Alternative: Direct initialization
    // userStore := user.NewUserStore()
    // productStore := product.NewProductStore()

    // Set up routes that use your stores
    app.GET("/users/{id}", func(ctx *gofr.Context) (interface{}, error) {
        id := ctx.PathParam("id")
        userID, _ := strconv.ParseInt(id, 10, 64)
        
        return userStore.GetUserByID(ctx, userID)
    })

    app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {
        return userStore.GetAllUsers(ctx)
    })

    app.POST("/users", func(ctx *gofr.Context) (interface{}, error) {
        var req struct {
            Name  string `json:"name"`
            Email string `json:"email"`
        }
        
        if err := ctx.Bind(&req); err != nil {
            return nil, err
        }
        
        return userStore.CreateUser(ctx, req.Name, req.Email)
    })

    app.GET("/products/{id}", func(ctx *gofr.Context) (interface{}, error) {
        id := ctx.PathParam("id")
        productID, _ := strconv.ParseInt(id, 10, 64)
        
        return productStore.GetProductByID(ctx, productID)
    })

    app.Start()
}
```

### Dependency Injection Pattern

For more advanced dependency injection, you can use the store registry:

```go
package main

import (
    "gofr.dev/pkg/gofr"
    "your-project/stores"
)

type Services struct {
    UserStore    interface{}
    ProductStore interface{}
    // Add more stores as needed
}

func main() {
    app := gofr.New()

    // Initialize all stores from registry
    storeRegistry := stores.All()
    
    // Create services container
    services := &Services{
        UserStore:    storeRegistry["user"](),
        ProductStore: storeRegistry["product"](),
    }

    // Set up handlers with dependency injection
    setupRoutes(app, services)

    app.Start()
}

func setupRoutes(app *gofr.Gofr, services *Services) {
    // User routes
    app.GET("/users/{id}", func(ctx *gofr.Context) (interface{}, error) {
        return handleGetUser(ctx, services.UserStore)
    })

    // Product routes
    app.GET("/products/{id}", func(ctx *gofr.Context) (interface{}, error) {
        return handleGetProduct(ctx, services.ProductStore)
    })
}

func handleGetUser(ctx *gofr.Context, store interface{}) (interface{}, error) {
    userStore := store.(user.UserStore) // Type assertion
    id := ctx.PathParam("id")
    userID, _ := strconv.ParseInt(id, 10, 64)
    
    return userStore.GetUserByID(ctx, userID)
}
```

### Environment-Specific Store Configuration

You can also set up different store configurations for different environments:

```go
package main

import (
    "os"
    "gofr.dev/pkg/gofr"
    "your-project/stores"
)

func main() {
    app := gofr.New()

    // Environment-based store initialization
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = "development"
    }

    // Initialize stores based on environment
    var storeRegistry map[string]func() any
    
    switch env {
    case "production":
        storeRegistry = stores.All() // Use generated stores
    case "testing":
        storeRegistry = getTestStores() // Use test/mock stores
    default:
        storeRegistry = stores.All() // Development
    }

    // Set up your application with the appropriate stores
    setupApplication(app, storeRegistry)

    app.Start()
}

func getTestStores() map[string]func() any {
    // Return mock/test store implementations
    return map[string]func() any{
        "user": func() any {
            return &MockUserStore{} // Your test implementation
        },
        "product": func() any {
            return &MockProductStore{} // Your test implementation
        },
    }
}

func setupApplication(app *gofr.Gofr, storeRegistry map[string]func() any) {
    // Use stores from registry to set up your routes
    userStore := storeRegistry["user"]()
    productStore := storeRegistry["product"]()
    
    // Set up routes...
}
```

## Store Registry Features

The store generator includes advanced registry management:

### Proper Appending Logic
- **Non-destructive updates**: New stores are appended to existing `stores/all.go` without overwriting
- **Duplicate prevention**: Automatically detects and skips existing stores
- **Import management**: Only adds new import statements for stores that don't already exist
- **Fallback regeneration**: If parsing fails, regenerates the complete file while preserving existing stores

### Registry File Structure

The generated `stores/all.go` provides:

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package stores

import (
    "your-project/stores/user"
    "your-project/stores/product"
)

// All returns all available store implementations
func All() map[string]func() any {
    return map[string]func() any {
        "user": func() any {
            return user.NewUserStore()
        },
        "product": func() any {
            return product.NewProductStore()
        },
    }
}

// GetStore returns a specific store by name
func GetStore(name string) any {
    stores := All()
    if storeFunc, exists := stores[name]; exists {
        return storeFunc()
    }
    return nil
}
```

## Multi-Store Benefits

The multi-store approach provides several advantages:

- **Separation of Concerns**: Each store handles its own domain (users, products, orders, etc.)
- **Independent Development**: Teams can work on different stores independently
- **Selective Model Generation**: Only generates models that are actually used by each store
- **Optimized Imports**: Only imports external packages that are needed by each store
- **Mixed Model Support**: Can combine external and generated models in the same project
- **Scalable Architecture**: Easy to add new stores as your application grows
- **Registry Management**: Centralized store registry with proper append functionality

## YAML Configuration

### Multi-Store Structure (Recommended)

```yaml
version: "1.0"

# Shared models across all stores
models:
  # External model (referenced from existing file)
  - name: "User"
    path: "../structs/user.go"
    package: "your-project/structs"
  
  # Generated model (will be created)
  - name: "Product"
    fields:
      - name: "ID"
        type: "int64"
        tag: "db:\"id\" json:\"id\""
      - name: "Name"
        type: "string"
        tag: "db:\"name\" json:\"name\""
      - name: "Price"
        type: "float64"
        tag: "db:\"price\" json:\"price\""
      - name: "Description"
        type: "string"
        tag: "db:\"description\" json:\"description\""
      - name: "CreatedAt"
        type: "time.Time"
        tag: "db:\"created_at\" json:\"created_at\""

# Multiple stores configuration
stores:
  - name: "user"
    package: "user"
    output_dir: "stores/user"
    interface: "UserStore"
    implementation: "userStore"
    queries:
      - name: "GetUserByID"
        sql: "SELECT id, name, email, created_at, updated_at FROM users WHERE id = ?"
        type: "select"
        model: "User"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
        description: "Retrieves a user by their ID"

      - name: "GetAllUsers"
        sql: "SELECT id, name, email, created_at, updated_at FROM users ORDER BY created_at DESC"
        type: "select"
        model: "User"
        returns: "multiple"
        description: "Retrieves all users ordered by creation date"

      - name: "CreateUser"
        sql: "INSERT INTO users (name, email, created_at, updated_at) VALUES (?, ?, NOW(), NOW())"
        type: "insert"
        params:
          - name: "name"
            type: "string"
          - name: "email"
            type: "string"
        description: "Creates a new user"

  - name: "product"
    package: "product"
    output_dir: "stores/product"
    interface: "ProductStore"
    implementation: "productStore"
    queries:
      - name: "GetProductByID"
        sql: "SELECT id, name, price, description, created_at FROM products WHERE id = ?"
        type: "select"
        model: "Product"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
        description: "Retrieves a product by its ID"

      - name: "GetProductsByPriceRange"
        sql: "SELECT id, name, price, description, created_at FROM products WHERE price BETWEEN ? AND ? ORDER BY price ASC"
        type: "select"
        model: "Product"
        returns: "multiple"
        params:
          - name: "minPrice"
            type: "float64"
          - name: "maxPrice"
            type: "float64"
        description: "Retrieves products within a price range"
```

### Single Store Structure (Legacy Support)

```yaml
version: "1.0"

store:
  package: "store"
  output_dir: "stores"
  interface: "Store"
  implementation: "store"

models:
  - name: "User"
    fields:
      - name: "ID"
        type: "int64"
        tag: "db:\"id\" json:\"id\""
      - name: "Name"
        type: "string"
        tag: "db:\"name\" json:\"name\""

queries:
  - name: "GetUserByID"
    sql: "SELECT id, name FROM users WHERE id = ?"
    type: "select"
    model: "User"
    returns: "single"
    params:
      - name: "id"
        type: "int64"
```

### Store Configuration

- `name`: Store name (used for directory structure and registry)
- `package`: Go package name for generated files
- `output_dir`: Directory where generated files will be created
- `interface`: Name of the store interface
- `implementation`: Name of the store implementation struct
- `queries`: Array of queries specific to this store

### Models

You can either generate new models or reference existing ones:

#### Option 1: Generate New Models

```yaml
models:
  - name: "User"
    fields:
      - name: "ID"
        type: "int64"
        tag: "db:\"id\" json:\"id\""
      - name: "Name"
        type: "string"
        tag: "db:\"name\" json:\"name\""
      - name: "Email"
        type: "string"
        tag: "db:\"email\" json:\"email\""
      - name: "CreatedAt"
        type: "time.Time"
        tag: "db:\"created_at\" json:\"created_at\""
      - name: "UpdatedAt"
        type: "time.Time"
        tag: "db:\"updated_at\" json:\"updated_at\""
```

#### Option 2: Reference Existing Models

```yaml
models:
  - name: "User"
    path: "../structs/user.go"
    package: "your-project/structs"
```

When using external models:
- `path`: Relative path to the model file (used for documentation)
- `package`: Full package path for import statements
- The generator will automatically add the necessary imports
- Generated code will use `package.Model` syntax (e.g., `structs.User`)

### Queries

Define your database queries:

```yaml
queries:
  - name: "GetUserByID"
    sql: "SELECT id, name FROM users WHERE id = ?"
    type: "select"          # select, insert, update, delete
    model: "User"           # Model to use for results
    returns: "single"       # single, multiple, count
    params:
      - name: "id"
        type: "int64"
    description: "Retrieves a user by their ID"
```

#### Query Types

- **select**: For SELECT queries
- **insert**: For INSERT queries
- **update**: For UPDATE queries
- **delete**: For DELETE queries
- **transaction**: For transaction-wrapped operations
- **health**: For health check operations

#### Return Types

- **single**: Returns a single model instance
- **multiple**: Returns a slice of model instances
- **count**: Returns an int64 count
- **custom**: Returns interface{} (for custom return types)

## Generated Files

The generator creates the following files:

### Per Store Directory
1. **interface.go**: Contains the store interface definition with all method signatures
2. **<implementation_name>.go**: Contains the store implementation with method stubs (e.g., userStore.go)
3. **<model_name>.go**: Individual model files (e.g., user.go, product.go) - only created when generating new models

### At Stores Root Level
4. **all.go**: Contains the store registry for dependency injection (generated at `stores/all.go`)

### Multi-Store File Structure Example

```
stores/
├── all.go                 # Store registry (all stores)
├── user/
│   ├── interface.go        # UserStore interface
│   ├── userStore.go        # UserStore implementation
│   └── store.yaml         # Configuration (from init command)
└── product/
    ├── interface.go        # ProductStore interface
    ├── productStore.go     # ProductStore implementation
    ├── product.go         # Product model (if generated)
    └── store.yaml         # Configuration (from init command)
```

### Key Features

- **Store Isolation**: Each store only contains files relevant to its own models and queries
- **Smart Model Generation**: Only generates model files that are actually used by each store
- **Optimized Imports**: Only imports external model packages that are actually used by each store
- **Mixed Model Support**: Can mix external and generated models in the same configuration
- **Registry Management**: Centralized store registry with proper appending and duplicate prevention

## Example Generated Code

### Interface (with External Models)

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package user

import (
    "gofr.dev/pkg/gofr"
    "your-project/structs"
)

// UserStore defines the interface for store operations
type UserStore interface {
    GetUserByID(ctx *gofr.Context, id int64) (structs.User, error)
    GetAllUsers(ctx *gofr.Context) ([]structs.User, error)
    CreateUser(ctx *gofr.Context, name string, email string) (any, error)
    UpdateUser(ctx *gofr.Context, name string, email string, id int64) (int64, error)
    DeleteUser(ctx *gofr.Context, id int64) (int64, error)
}
```

### Implementation

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package user

import (
    "gofr.dev/pkg/gofr"
    "your-project/structs"
)

// userStore implements the UserStore interface
type userStore struct {
    // Add any dependencies here (e.g., database connection)
}

// NewUserStore creates a new instance of UserStore
func NewUserStore() UserStore {
    return &userStore{}
}

// GetUserByID retrieves a user by their ID
func (s *userStore) GetUserByID(ctx *gofr.Context, id int64) (structs.User, error) {
    // TODO: Implement GetUserByID query
    // SQL: SELECT id, name, email, created_at, updated_at FROM users WHERE id = ?
    
    var result structs.User
    // Implement single row selection using ctx.SQL()
    // Example: err := ctx.SQL().QueryRowContext(ctx, 
    // "SELECT id, name, email, created_at, updated_at FROM users WHERE id = ?", 
    // id).Scan(&result.ID, &result.Name, &result.Email, &result.CreatedAt, &result.UpdatedAt)
    return result, nil
}
```

### Generated Model

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package product

import (
    "time"
)

// Product represents the Product model
type Product struct {
    ID          int64     `db:"id" json:"id"`
    Name        string    `db:"name" json:"name"`
    Price       float64   `db:"price" json:"price"`
    Description string    `db:"description" json:"description"`
    CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// TableName returns the table name for Product
func (Product) TableName() string {
    return "product"
}
```

## Advanced Features

### Robust Appending Logic

The store generator uses sophisticated logic to handle the `stores/all.go` registry:

1. **File Parsing**: Reads existing registry and extracts current stores and imports
2. **Duplicate Detection**: Prevents duplicate store entries and import statements
3. **Import Management**: Only adds imports for stores that don't already exist
4. **Multiple Insertion Strategies**: Uses multiple methods to find correct insertion points
5. **Fallback Regeneration**: If parsing fails, regenerates the complete file while preserving existing stores

### Error Handling and Recovery

- **Graceful Degradation**: If specific insertion points can't be found, falls back to complete file regeneration
- **Comprehensive Logging**: Detailed logging for troubleshooting append operations
- **Validation**: Validates existing file structure before attempting modifications
- **Backup Strategy**: Preserves existing store information during regeneration

### Linter Compliance

The generated code passes all major Go linters:

- **golangci-lint**: Full compliance with all enabled checks
- **gofmt**: Proper code formatting
- **govet**: Static analysis compliance
- **gocyclo**: Low cyclomatic complexity
- **Clean imports**: Optimized import statements
- **Consistent naming**: Following Go naming conventions

## Implementation Notes

After generation, you need to implement the actual database logic in the generated methods. The generator provides:

- Method signatures with proper GoFr context support
- SQL query comments for reference
- Example code comments showing how to use `ctx.SQL()`
- Proper error handling structure
- Clean, linter-compliant code structure

## Best Practices

1. **Use descriptive query names**: Make method names clear and descriptive
2. **Include descriptions**: Add descriptions to your queries for better documentation
3. **Consistent naming**: Use consistent naming conventions for models and fields
4. **Proper tags**: Include appropriate struct tags for database and JSON serialization
5. **Type safety**: Use proper Go types that match your database schema
6. **External model organization**: Keep external models in dedicated packages (e.g., `structs/`, `models/`)
7. **Configuration management**: Use separate config files for different environments

## Testing

The store generator has been comprehensively tested with the following scenarios:

### ✅ Test Case 1: External Models (Multi-store YAML with model paths)
- **Status**: PASSED
- **Features**: Multiple stores using external models from `structs/` package
- **Generated**: `stores/user/` and `stores/product/` with correct external model references
- **Imports**: Correctly imports external model packages
- **Registry**: Properly appends to `stores/all.go` without overwriting

### ✅ Test Case 2: Generated Models (Multi-store YAML without model paths)
- **Status**: PASSED
- **Features**: Multiple stores with generated model structs
- **Generated**: `stores/order/` and `stores/payment/` with generated `Order` and `Payment` models
- **Models**: Properly generated with correct field types and tags
- **Registry**: Creates and maintains store registry

### ✅ Test Case 3: Mixed Models (One external, one generated)
- **Status**: PASSED
- **Features**: User store uses external `structs.User`, Product store generates `Product` model
- **Generated**: Correct isolation - each store only imports what it needs
- **Compilation**: All code compiles successfully
- **Registry**: Handles mixed store types correctly

### ✅ Test Case 4: Append Functionality
- **Status**: PASSED
- **Features**: Multiple sequential store generations append correctly
- **Registry**: No duplicate entries, proper import management
- **Robustness**: Handles various formatting and edge cases
- **Recovery**: Graceful fallback when parsing fails

### ✅ Test Case 5: Linter Compliance
- **Status**: PASSED
- **Tools**: golangci-lint, gofmt, govet, gocyclo
- **Coverage**: All generated code passes strict linting rules
- **Standards**: Follows Go best practices and conventions

### ✅ Additional Features Tested:
- **Multi-store generation**: Single YAML generates multiple stores
- **Store isolation**: Each store only generates relevant files
- **Import optimization**: Only imports packages that are actually used
- **Code compilation**: All generated code compiles without errors
- **Template syntax**: Proper function signature formatting with clean spacing
- **Model generation**: Both external references and generated models work correctly
- **Registry management**: Robust append functionality with duplicate prevention

## Troubleshooting

### Common Issues

1. **Import Path Issues**: Ensure your `go.mod` module name matches the package paths in your YAML configuration
2. **Model Not Found**: Verify external model paths are correct and accessible
3. **Registry Conflicts**: If `stores/all.go` becomes corrupted, delete it and regenerate
4. **Permission Issues**: Ensure write permissions for the `stores/` directory

### Debug Mode

Enable detailed logging by setting the GoFr log level to debug mode to see detailed information about the generation process.

## Example Usage

See the provided YAML configuration examples for complete configurations with multiple models and queries. The generator supports both simple single-store setups and complex multi-store architectures with mixed model types.