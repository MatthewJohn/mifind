# mifind Implementation Design

## Overview

This document captures the implementation design for **mifind** - a unified personal search tool that federates queries across multiple data providers with a pluggable architecture.

---

## Architecture Principles

1. **Source of Truth**: Providers are authoritative; mifind only indexes lightweight representations
2. **No Search Engine Initially**: Rely on provider APIs with custom ranking/federation
3. **Future Caching**: Meilisearch for faceted aggregations and performance
4. **Pluggable Providers**: New providers and types can be added without core changes
5. **AI/Agent Friendly**: Structured API via MCP for AI agent integration

---

## Project Structure

```
mifind/
├── cmd/
│   ├── mifind/
│   │   └── main.go              # HTTP API server entry point
│   ├── mifind-mcp/
│   │   └── main.go              # MCP server entry point (separate binary)
│   └── filesystem-api/
│       └── main.go              # Standalone filesystem API service
├── internal/
│   ├── mifind/
│   │   ├── server.go            # HTTP server setup
│   │   └── config.go            # Configuration loading
│   ├── search/
│   │   ├── federator.go         # Federates queries to providers
│   │   ├── ranker.go            # Result ranking/scoring engine
│   │   ├── filters.go           # Adaptive filter logic
│   │   └── relationships.go     # Relationship traversal
│   ├── types/
│   │   ├── entity.go            # Entity/Item definition
│   │   ├── type_registry.go     # Type system and hierarchy
│   │   ├── attributes.go        # Attribute definitions
│   │   └── relationships.go     # Relationship types
│   ├── provider/
│   │   ├── interface.go         # Provider interface definition
│   │   ├── registry.go          # Provider plugin registry
│   │   ├── manager.go           # Provider lifecycle manager
│   │   └── mock/                # Test provider
│   ├── cache/
│   │   ├── cache.go             # Cache interface and implementation
│   │   └── eviction.go          # TTL/LRU eviction policies
│   └── api/
│       ├── handlers.go          # HTTP handlers
│       └── mcp_tools.go         # MCP tool implementations
├── pkg/
│   ├── provider/                # Provider implementations (call external APIs)
│   │   ├── filesystem/          # Filesystem provider (calls filesystem-api)
│   │   │   ├── provider.go
│   │   │   ├── client.go        # HTTP client for filesystem-api
│   │   │   └── types.go         # API type mappings
│   │   └── immich/              # Immich provider (photos/videos with geo/ML)
│   │       ├── provider.go
│   │       ├── client.go        # HTTP client for Immich API
│   │       └── types.go         # API type mappings
│   └── shared/                  # Shared code between mifind and filesystem-api
│       ├── types/               # Common entity/type definitions
│       └── util/                # Shared utilities
├── go.mod
├── go.sum
└── README.md
```

---

## Core Data Model

### Entity

```go
type Entity struct {
    ID            string           // Stable, provider-scoped ID
    Type          string           // Hierarchical: "file.media.video"
    Provider      string           // Source provider name
    Title         string           // Display name
    Description   string           // Optional description
    Attributes    map[string]any   // Typed values for filtering/display
    Relationships []Relationship   // Optional: connections to other entities
    SearchTokens  []string         // Flattened text for full-text search
    Timestamp     time.Time        // Cache/last-seen timestamp
}

type Relationship struct {
    Type     string  // "album", "folder", "person", etc.
    TargetID string  // ID of related entity
}
```

### Type System

Hierarchical types with inheritance:

```
Item
├── File
│   ├── MediaFile
│   │   ├── ImageFile
│   │   └── VideoFile
│   └── DocumentFile
├── MediaAsset
│   ├── PhotoAsset
│   └── VideoAsset
├── Collection
│   ├── Album
│   └── Folder
└── Person
```

---

## Provider Interface

```go
type Provider interface {
    Name() string
    Initialize(config map[string]any) error
    Discover(ctx context.Context) ([]Entity, error)
    Hydrate(ctx context.Context, id string) (Entity, error)
    GetRelated(ctx context.Context, id string, relType string) ([]Entity, error)
    Search(ctx context.Context, query SearchQuery) ([]Entity, error)
    SupportsIncremental() bool
    Shutdown() error
}

type SearchQuery struct {
    Query    string
    Filters  map[string]any
    Limit    int
    Offset   int
}
```

---

## Search Architecture

### Federation

The search federator broadcasts queries to all providers and aggregates results:

1. Send query to all providers concurrently
2. Collect results with timeouts/failure handling
3. Pass to ranker for scoring and deduplication
4. Return unified results with type counts

### Ranking

The ranker scores and orders results:

- **Relevance scoring**: Based on provider's relevance + mifind's type weights
- **Deduplication**: Handle duplicates across providers (e.g., Immich has many similar photos)
- **Type boosting**: Configure preferred types per query
- **Pagination**: Support offset/limit across providers

### Filters

Adaptive filtering based on result types:

1. Extract available filters from aggregated results
2. Group by attribute with counts
3. Return dynamic filter options to UI
4. Apply filters (delegate to providers when possible)

---

## API Endpoints

### HTTP API

- `POST /search` - Search with optional filters
- `GET /entity/:id` - Get entity details with relationships
- `GET /types` - List all types with hierarchy
- `GET /filters?search=...` - Get available filters for results

### MCP Tools

- `search_entities(query, filters)` - Search and return entities
- `describe_entity(id)` - Get full entity details
- `expand_entity(id)` - Get entity with related entities

---

## Provider Implementations

### Filesystem API with Full-Text Search

The filesystem API is a standalone service that provides full-text search over filesystem metadata:

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   mifind API    │──────│ Filesystem      │──────│  Filesystem     │
│   (HTTP/MCP)    │      │ Provider        │      │  API Service    │
└─────────────────┘      │ (client code)   │      └─────────────────┘
                         └─────────────────┘              │
                                                        │ scans + indexes
                                                        ▼
                                             ┌─────────────────────┐
                                             │  Isolated Meilisearch│
                                             │  (full-text search) │
                                             └─────────────────────┘
```

**Why Meilisearch for filesystem-api?**
- Directory scanning on every request is expensive
- Full-text search over filenames/paths requires a fast index
- Tokenization, normalization, partial matches (as per DESIGN.md Search Model)
- Fast faceted aggregations (file types, extensions, sizes)
- Incremental updates: track modified files by timestamp/inode

**Isolation**: This is a completely isolated Meilisearch instance
- Not shared with mifind core
- Can be rebuilt/restarted without affecting other services
- Each filesystem-api deployment has its own Meilisearch instance

**Search capabilities** (per DESIGN.md):
- Full-text search across indexed item summaries
- Tokenization & normalization: filename, extension, path
- Partial matches, typo tolerance
- Two-phase adaptive search: broad then narrow by filters

### Architecture Pattern

All providers follow the same pattern: they are **external APIs** that mifind calls via a provider client. This includes the filesystem provider, which is built within this project but runs as a separate service.

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   mifind API    │──────│ Filesystem      │──────│  Filesystem     │
│   (HTTP/MCP)    │      │ Provider        │      │  API Service    │
└─────────────────┘      │ (client code)   │      └─────────────────┘
                         └─────────────────┘              │
                                                        │ scans
                                                        ▼
                                                   actual files
```

### Filesystem Provider

The filesystem provider consists of two parts:

1. **`cmd/filesystem-api/`** - Standalone HTTP API service:
   - Scans configured directory paths and indexes them
   - Full-text search using isolated database (SQLite FTS5 or Tantivy)
   - Exposes REST endpoints: `/search`, `/browse`, `/file/:id`
   - Handles file type detection, metadata extraction
   - Supports incremental updates via file modification tracking
   - Reuses shared types/utilities from `pkg/shared/`

2. **`pkg/provider/filesystem/`** - mifind provider client:
   - Calls the filesystem-api via HTTP
   - Maps API responses to internal Entity types
   - Handles connection, authentication, retries

**Types**: File, MediaFile, DocumentFile
**Attributes**: path, size, extension, mime_type, modified_time
**Relationships**: parent_folder, children

### Immich Provider

- **Connects**: Immich REST API
- **Types**: MediaAsset, PhotoAsset, VideoAsset, Album, Person, Place
- **Attributes**: camera, lens, iso, gps, faces, exif data
- **Relationships**: album→assets, person→assets, place→assets
- **Why Immich**: Rich metadata, duplicates, pagination testing, ML search

---

## Configuration

```yaml
server:
  http_port: 8080
  mcp_port: 8081

cache:
  ttl: 3600
  max_items: 10000

providers:
  filesystem:
    paths:
      - /home/user/Documents
    excluded_dirs: [".git", "node_modules"]

  immich:
    url: "https://immich.example.com"
    api_key: "${IMMICH_API_KEY}"
```

---

## Dependencies

- **HTTP**: `net/http` + `github.com/gorilla/mux`
- **MCP**: `github.com/modelcontextprotocol/sdk-go`
- **Logging**: `github.com/rs/zerolog`
- **Config**: `github.com/spf13/viper`
- **Testing**: `testing` + `github.com/stretchr/testify`
- **Meilisearch** (filesystem-api only): `github.com/meilisearch/meilisearch-go`

---

## Open Design Questions

### Attribute Unification Across Providers

**Problem**: How should attributes be defined and shared across providers?

#### Scenario Examples

1. **Common concepts, different names**:
   - Filesystem provider has: `path`, `size`, `modified`
   - Immich provider has: `originalPath`, `fileSize`, `fileCreatedAt`
   - GitLab provider (future) has: `filePath`, `size`, `updatedAt`
   - These are conceptually similar but named differently

2. **Provider-specific attributes**:
   - Immich has: `iso`, `lensModel`, `faceCount`, `smartInfo`
   - Filesystem has: `inode`, `permissions`, `owner`
   - These are unique to each provider

3. **Future provider needs**:
   - Adding a Jellyfin provider should be possible without modifying core
   - But Jellyfin's `duration` should be "the same" as Immich's `duration`

#### Approaches to Consider

**Option A: Core + Provider-scoped Attributes**

Core defines standard attributes with types. Providers map their fields:

```go
// Core attributes
const (
    AttrPath       = "path"
    AttrSize       = "size"
    AttrModified   = "modified"
    AttrDuration   = "duration"
    AttrGPS        = "gps"
)

// Provider can add provider-scoped attributes
const (
    AttrImmichFaceCount = "immich:faceCount"
    AttrImmichSmartInfo = "immich:smartInfo"
)
```

- **Pros**: Consistent filtering, type-safe, clear what's standard
- **Cons**: Adding new standard attributes requires core changes
- **Extension**: Providers add prefixed attributes for unique fields

**Option B: Provider-defined with Aliasing**

Providers define their own attributes, but can alias to common concepts:

```go
// Provider declares attribute mappings
func (p *FilesystemProvider) GetAttributeMappings() map[string]string {
    return map[string]string{
        "path":     "path",        // Direct mapping to standard
        "size":     "size",        // Direct mapping
        "modified": "modified",    // Direct mapping
    }
}

func (p *ImmichProvider) GetAttributeMappings() map[string]string {
    return map[string]string{
        "originalPath":   "path",       // Alias to standard
        "fileSize":       "size",       // Alias to standard
        "exifInfo.iso":   "iso",        // Provider-specific
        "smartInfo":      "smartInfo",  // Provider-specific
    }
}
```

- **Pros**: Easy to add providers, flexible naming
- **Cons**: Need to handle type mismatches, more complex filtering logic

**Option C: Interface-based Attribute Registry**

Attributes are defined with interfaces for type safety and extensibility:

```go
type AttributeDefinition struct {
    Name        string
    Type        AttributeType  // String, Int, Float, GPS, etc.
    Description string
    Filterable  bool
}

type AttributeRegistry interface {
    Register(def AttributeDefinition)
    Get(name string) (AttributeDefinition, bool)
    GetByProvider(provider string) []AttributeDefinition
    // Unification: find equivalent attributes across providers
    FindEquivalent(attr string, providers []string) []string
}
```

- **Pros**: Type-safe, extensible, supports unification
- **Cons**: More complex implementation

#### Questions to Consider

1. **Should adding a new provider require modifying core code?**
   - If yes: Core attributes can grow over time
   - If no: Need a mechanism for providers to declare new attributes

2. **How should filtering work on provider-specific attributes?**
   - Should they be filterable at all?
   - Should the UI show them differently?

3. **How do we handle type mismatches?**
   - Immich's `duration` is seconds, Jellyfin's is milliseconds
   - GPS coordinates: different formats

4. **Should there be "standard" vs "extended" attributes?**
   - Standard: Defined in core, all providers should map if applicable
   - Extended: Provider-specific, not guaranteed across providers

---

## Verification Steps

1. Initialize project: `go mod init github.com/yourname/mifind`
2. Build core types: Run unit tests for entity and type registry
3. Test provider interface: Create mock provider, run discovery
4. Test federator: Query multiple mock providers, verify aggregation
5. Test ranker: Verify ranking, deduplication, pagination
6. HTTP API test: `curl http://localhost:8080/search?q=test`
7. MCP server test: Run `mifind-mcp`, verify tools available
8. Filesystem provider: Scan directory, verify entities discovered
9. Immich provider: Connect to Immich, fetch assets/albums
10. End-to-end: Search across both providers, verify unified results
