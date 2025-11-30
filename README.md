> Status: initial draft v0.1, not for production use

# GTS Go Library

A minimal, idiomatic Go library for working with **GTS** ([Global Type System](https://github.com/gts-spec/gts-spec)) identifiers and JSON/JSON Schema artifacts.

## Roadmap

Featureset:

- [x] **OP#1 - ID Validation**: Verify identifier syntax using regex patterns
- [x] **OP#2 - ID Extraction**: Fetch identifiers from JSON objects or JSON Schema documents
- [x] **OP#3 - ID Parsing**: Decompose identifiers into constituent parts (vendor, package, namespace, type, version, etc.)
- [x] **OP#4 - ID Pattern Matching**: Match identifiers against patterns containing wildcards
- [x] **OP#5 - ID to UUID Mapping**: Generate deterministic UUIDs from GTS identifiers
- [x] **OP#6 - Schema Validation**: Validate object instances against their corresponding schemas
- [x] **OP#7 - Relationship Resolution**: Load all schemas and instances, resolve inter-dependencies, and detect broken references
- [x] **OP#8 - Compatibility Checking**: Verify that schemas with different MINOR versions are compatible
- [x] **OP#8.1 - Backward compatibility checking**
- [x] **OP#8.2 - Forward compatibility checking**
- [x] **OP#8.3 - Full compatibility checking**
- [x] **OP#9 - Version Casting**: Transform instances between compatible MINOR versions
- [x] **OP#10 - Query Execution**: Filter identifier collections using the GTS query language
- [x] **OP#11 - Attribute Access**: Retrieve property values and metadata using the attribute selector (`@`)

TODO - need a file with Go code snippets for all Ops above

Other GTS spec [Reference Implementation](https://github.com/globaltypesystem/gts-spec/blob/main/README.md#9-reference-implementation-recommendations) recommended features support:

- [x] **In-memory entities registry** - simple GTS entities registry with optional GTS references validation on entity registration
- [x] **CLI** - command-line interface for all GTS operations
- [x] **Web server** - a non-production web-server with REST API for the operations processing and testing
- [x] **x-gts-ref support** - to support special GTS entity reference annotation in schemas
- [ ] **YAML support** - to support YAML files (*.yml, *.yaml) as input files
- [ ] **TypeSpec support** - add [typespec.io](https://typespec.io/) files (*.tsp) support
- [ ] **UUID for instances** - to support UUID as ID in JSON instances

Technical Backlog:

- [ ] **Code coverage** - target is 90%
- [ ] **Documentation** - add documentation for all the features
- [ ] **Interface** - export publicly available interface and keep cli and others private
- [ ] **Final code cleanup** - remove unused code, denormalize, add critical comments, etc.

## Installation

```bash
go get github.com/GlobalTypeSystem/gts-go
```

## Usage

### Library

Import the GTS package in your Go code:

```go
import "github.com/GlobalTypeSystem/gts-go/gts"
```

#### OP#1 - ID Validation

```go
// Validate a GTS ID
if gts.IsValidGtsID("gts.vendor.pkg.ns.type.v1~") {
    fmt.Println("Valid GTS ID")
}

// Get detailed validation result
result := gts.ValidateGtsID("gts.vendor.pkg.ns.type.v1~")
if result.Valid {
    fmt.Printf("Valid: %s\n", result.ID)
} else {
    fmt.Printf("Invalid: %s\n", result.Error)
}
```

#### OP#2 - ID Extraction

```go
// Extract GTS ID from JSON content
content := map[string]any{
    "gtsId": "gts.vendor.pkg.ns.type.v1.0",
    "name":  "My Entity",
}

result := gts.ExtractID(content, nil)
fmt.Printf("ID: %s\n", result.ID)
fmt.Printf("Schema ID: %s\n", result.SchemaID)
```

#### OP#3 - ID Parsing

```go
// Parse a GTS ID into segments
result := gts.ParseGtsID("gts.vendor.pkg.ns.type.v1~")
if result.OK {
    for _, seg := range result.Segments {
        fmt.Printf("Vendor: %s, Package: %s, Type: %s, Version: %d\n",
            seg.Vendor, seg.Package, seg.Type, seg.VerMajor)
    }
}
```

#### OP#4 - Pattern Matching

```go
// Match GTS ID against a pattern
result := gts.MatchIDPattern(
    "gts.vendor.pkg.ns.type.v1.0",
    "gts.vendor.pkg.*",
)
if result.Match {
    fmt.Println("Pattern matched!")
}
```

#### OP#5 - UUID Generation

```go
// Generate deterministic UUID from GTS ID
result := gts.IDToUUID("gts.vendor.pkg.ns.type.v1~")
fmt.Printf("UUID: %s\n", result.UUID)
```

#### Using the GTS Store

```go
// Create a new store
store := gts.NewGtsStore(nil)

// Register an entity
entity := gts.NewJsonEntity(map[string]any{
    "gtsId": "gts.vendor.pkg.ns.type.v1.0",
    "name":  "My Entity",
}, gts.DefaultGtsConfig())

err := store.Register(entity)
if err != nil {
    log.Fatal(err)
}

// Query entities
result := store.Query("gts.vendor.pkg.*", 100)
fmt.Printf("Found %d entities\n", result.Count)

// Validate an instance
validation := store.ValidateInstance("gts.vendor.pkg.ns.type.v1.0")
if validation.OK {
    fmt.Println("Instance is valid")
}

// Attribute access
attr := store.GetAttribute("gts.vendor.pkg.ns.type.v1.0@name")
if attr.Resolved {
    fmt.Printf("Attribute value: %v\n", attr.Value)
}
```

### CLI

The CLI provides command-line access to all GTS operations.

#### Installation

```bash
# Install the CLI tool
go install github.com/GlobalTypeSystem/gts-go/cmd/gts@latest

# Or build locally
go build -o gts ./cmd/gts
```

#### Usage

```bash
# Show help
gts

# Show version
gts version

# Command-specific help
gts <command> -h

# Basic operations (no file loading required)

# OP#1 - Validate a GTS ID
gts validate-id -id gts.vendor.pkg.ns.type.v1~

# OP#2 - Parse a GTS ID into components
gts parse-id -id gts.vendor.pkg.ns.type.v1.0

# OP#3 - Match ID against pattern
gts match-id -pattern "gts.vendor.pkg.*" -candidate gts.vendor.pkg.ns.type.v1.0

# OP#4 - Generate UUID from GTS ID
gts uuid -id gts.vendor.pkg.ns.type.v1~

# Operations that require loading files (use -path flag)

# OP#5 - Validate instance against schema
gts -path ./examples validate -id gts.vendor.pkg.ns.type.v1.0

# OP#6 - Resolve relationships
gts -path ./examples relationships -id gts.vendor.pkg.ns.type.v1~

# OP#7 - Check schema compatibility
gts -path ./examples compatibility \
  -old gts.vendor.pkg.ns.type.v1~ \
  -new gts.vendor.pkg.ns.type.v2~

# OP#8 - Cast instance to different schema version
gts -path ./examples cast \
  -from gts.vendor.pkg.ns.type.v1.0 \
  -to gts.vendor.pkg.ns.type.v2~

# OP#9 - Query entities
gts -path ./examples query -expr "gts.vendor.pkg.*" -limit 10

# OP#10 - Get attribute value
gts -path ./examples attr -path gts.vendor.pkg.ns.type.v1.0@name

# List all entities
gts -path ./examples list -limit 100

# Start HTTP server
gts -path ./examples server -host 127.0.0.1 -port 8000

# Generate OpenAPI specification
gts openapi -out openapi.json
```

#### Global Flags

```bash
# Verbose logging
gts -v -path ./examples list

# Custom config file
gts -config ./gts.config.json -path ./examples list
```

#### Environment Variables

The CLI supports the following environment variables:

- `GTS_PATH` - Default path to JSON and schema files
- `GTS_CONFIG` - Default path to GTS config JSON file
- `GTS_VERBOSE` - Default verbosity level (0, 1, or 2)

Example:

```bash
export GTS_PATH=./examples
export GTS_VERBOSE=1
gts list
```

#### Multiple Paths

You can load entities from multiple directories by separating paths with commas:

```bash
gts -path ./schemas,./instances list
```

### Library

TODO - See ...

### Web Server

The web server is a non-production web-server with REST API for the operations processing and testing. It implements reference API for gts-spec [tests](https://github.com/GlobalTypeSystem/gts-spec/tree/main/tests)

```bash
# Start the web server (default: http://127.0.0.1:8000)
gts --path ./examples server

# Start on a different port
gts --path ./examples server --host 127.0.0.1 --port 8001

# Pre-populate server with JSON instances and schemas from the gts-spec tests
# The server will automatically load all entities from the specified path
gts --path /path/to/gts-spec/tests/entities server

# View server logs
gts -v --path ./examples server

# Alternative: use the dedicated server binary
go run ./cmd/gts-server -host 127.0.0.1 -port 8000 -verbose 1
```

### Testing

You can test the gts-go library by utilizing the shared test suite from the [gts-spec](https://github.com/GlobalTypeSystem/gts-spec) specification and executing the tests against the web server.

Executing gts-spec Tests on the Server:

```bash
# getting the tests
git clone https://github.com/GlobalTypeSystem/gts-spec.git
cd gts-spec/tests

# run tests against the web server on port 8000 (default)
pytest

# override server URL using GTS_BASE_URL environment variable
GTS_BASE_URL=http://127.0.0.1:8001 pytest

# or set it persistently
export GTS_BASE_URL=http://127.0.0.1:8001
pytest
```

## License

Apache License 2.0
