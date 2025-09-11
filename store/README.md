# GoFr Store Generator

The GoFr Store Generator is a CLI tool that generates store layer functions with context support using YAML configuration files.

## Features

- **YAML Configuration**: Define your models and queries in a simple YAML file
- **Context Support**: All generated methods include `*gofr.Context` as the first parameter
- **Store Struct Pattern**: Follows GoFr's store layer structure with methods on store structs
- **Multiple Query Types**: Supports select, insert, update, delete, transaction, and health operations
- **Flexible Return Types**: Supports single, multiple, count, and custom return types
- **Model Generation**: Automatically generates Go structs for your data models
- **External Model Support**: Reference existing model files instead of generating new ones
- **GoFr Integration**: Uses `ctx.SQL()` for database operations with proper GoFr patterns

## Commands

### Initialize Store

Create a new store with initial configuration and directory structure:

```bash
gofr store init -name=<store_name>
```

This command:
- Creates the `stores/<store_name>` directory
- Generates a `store.yaml` configuration file with examples
- Creates initial `interface.go`, `<store_name>.go`, and `all.go` files
- Sets up the basic store structure

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

# Generate store code from configuration
gofr store generate -config=stores/user/store.yaml

# Generate multiple stores from a single configuration
gofr store generate -config=multi-store.yaml

# Generate with default config path
gofr store generate
```

## Multi-Store Benefits

The multi-store approach provides several advantages:

- **Separation of Concerns**: Each store handles its own domain (users, products, orders, etc.)
- **Independent Development**: Teams can work on different stores independently
- **Selective Model Generation**: Only generates models that are actually used by each store
- **Optimized Imports**: Only imports external packages that are needed by each store
- **Mixed Model Support**: Can combine external and generated models in the same project
- **Scalable Architecture**: Easy to add new stores as your application grows

## YAML Configuration

### Multi-Store Structure (Recommended)

```yaml
version: "1.0"

# Shared models across all stores
models:
  - name: "User"
    path: "models/user.go"
    package: "your-project/models"
  - name: "Product"
    fields:
      - name: "ID"
        type: "int64"
        tag: "db:\"id\" json:\"id\""
      - name: "Name"
        type: "string"
        tag: "db:\"name\" json:\"name\""

# Multiple stores configuration
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

  - name: "product"
    package: "product"
    output_dir: "stores/product"
    interface: "ProductStore"
    implementation: "productStore"
    queries:
      - name: "GetProductByID"
        sql: "SELECT id, name FROM products WHERE id = ?"
        type: "select"
        model: "Product"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
```

### Single Store Structure 

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

- `name`: Store name (used for directory structure)
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
    path: "models/user.go"
    package: "your-project/models"
```

When using external models:
- `path`: Relative path to the model file
- `package`: Full package path for import statements
- The generator will automatically add the necessary imports
- Generated code will use `package.Model` syntax (e.g., `models.User`)

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

The generator creates the following files for each store:

1. **interface.go**: Contains the store interface definition with all method signatures
2. **<implementation_name>.go**: Contains the store implementation with method stubs (e.g., userStore.go)
3. **<model_name>.go**: Individual model files (e.g., user.go, product.go) - only created when generating new models

### Multi-Store File Structure Example

```
stores/
├── user/
│   ├── interface.go        # UserStore interface
│   ├── userStore.go        # UserStore implementation
│   └── user.go            # User model (if generated)
└── product/
    ├── interface.go        # ProductStore interface
    ├── productStore.go     # ProductStore implementation
    └── product.go         # Product model (if generated)
```

### Key Features

- **Store Isolation**: Each store only contains files relevant to its own models and queries
- **Smart Model Generation**: Only generates model files that are actually used by each store
- **Optimized Imports**: Only imports external model packages that are actually used by each store
- **Mixed Model Support**: Can mix external and generated models in the same configuration

## Example Generated Code

### Interface (with External Models)

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package user

import (
	"gofr.dev/pkg/gofr"
	"your-project/models"
)

// UserStore defines the interface for store operations
type UserStore interface {
	GetUserByID(ctx *gofr.Context, id int64) (models.User, error)
	GetAllUsers(ctx *gofr.Context) ([]models.User, error)
	CreateUser(ctx *gofr.Context, name string, email string) (interface{}, error)
	UpdateUser(ctx *gofr.Context, name string, email string, id int64) (interface{}, error)
	DeleteUser(ctx *gofr.Context, id int64) (interface{}, error)
}
```

### Implementation

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package user

import (
	"gofr.dev/pkg/gofr"
	"your-project/models"
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
func (s *userStore) GetUserByID(ctx *gofr.Context, id int64) (models.User, error) {
	// TODO: Implement GetUserByID query
	// SQL: SELECT id, name, email, created_at, updated_at FROM users WHERE id = ?
	
	var result models.User
	// Implement single row selection using ctx.SQL()
	// Example: err := ctx.SQL().QueryRowContext(ctx, "SELECT id, name, email, created_at, updated_at FROM users WHERE id = ?", id).Scan(&result.ID, &result.Name, &result.Email, &result.CreatedAt, &result.UpdatedAt)
	return result, nil
}
```

### Model

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package store

import (
	"time"
)

// User represents the User model
type User struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// TableName returns the table name for User
func (User) TableName() string {
	return "user"
}
```

## Implementation Notes

After generation, you need to implement the actual database logic in the generated methods. The generator provides:

- Method signatures with proper GoFr context support
- SQL query comments for reference
- Example code comments showing how to use `ctx.SQL()`
- Proper error handling structure

## Best Practices

1. **Use descriptive query names**: Make method names clear and descriptive
2. **Include descriptions**: Add descriptions to your queries for better documentation
3. **Consistent naming**: Use consistent naming conventions for models and fields
4. **Proper tags**: Include appropriate struct tags for database and JSON serialization
5. **Type safety**: Use proper Go types that match your database schema

## Example Usage

See `example.yaml` for a complete example configuration with multiple models and queries.
