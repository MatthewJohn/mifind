# mifind API

Base URL: `http://localhost:8080`

---

## Search

### POST /search

Search across all providers for entities matching the query.

**Request body:**
```json
{
  "query": "vacation photos",
  "filters": {},
  "type": "",
  "limit": 20,
  "offset": 0,
  "type_weights": {},
  "include_related": false,
  "max_depth": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| `query` | string | Search query text |
| `filters` | object | Attribute filters (e.g., `{"extension": "jpg"}`) |
| `type` | string | Filter by entity type |
| `limit` | int | Max results (default: 20) |
| `offset` | int | Results to skip |
| `type_weights` | object | Boost weights by type |
| `include_related` | bool | Include related entities |
| `max_depth` | int | Max depth for related entities |

**Response:**
```json
{
  "entities": [
    {
      "id": "filesystem:myfs:abc123",
      "type": "file.media.image",
      "provider": "filesystem",
      "title": "vacation.jpg",
      "description": "",
      "attributes": {
        "path": "/home/user/Pictures/vacation.jpg",
        "extension": "jpg",
        "mime_type": "image/jpeg",
        "size": 2458624,
        "modified": 1703847600
      },
      "relationships": [],
      "search_tokens": [],
      "timestamp": "2024-01-01T00:00:00Z"
    }
  ],
  "total_count": 42,
  "type_counts": {"file.media.image": 15, "file.document": 10},
  "duration_ms": 23.5
}
```

---

### POST /search/federated

Search with per-provider results (useful for debugging or multi-source views).

**Request body:** Same as `/search`

**Response:**
```json
{
  "results": [
    {
      "provider": "filesystem",
      "entities": [...],
      "error": "",
      "duration_ms": 12.3,
      "type_counts": {"file.media.image": 15}
    }
  ],
  "total_count": 42,
  "type_counts": {"file.media.image": 15, "file.document": 10},
  "has_errors": false,
  "duration_ms": 23.5
}
```

---

## Entities

### GET /entity/{id}

Retrieve a single entity by ID.

**Response:**
```json
{
  "id": "filesystem:myfs:abc123",
  "type": "file.media.image",
  "provider": "filesystem",
  "title": "vacation.jpg",
  "attributes": {...},
  "relationships": [...]
}
```

---

### GET /entity/{id}/expand

Retrieve an entity with its relationships expanded.

**Query params:**
- `depth` (int): Max depth for expansion (default: 1)

**Response:**
```json
{
  "entity": {...},
  "related": [
    {
      "relationship": "folder",
      "entities": [...]
    }
  ]
}
```

---

### GET /entity/{id}/related

Get entities related to the given entity.

**Query params:**
- `type` (string): Relationship type filter
- `limit` (int): Max results

**Response:**
```json
{
  "entities": [...],
  "count": 5
}
```

---

## Types

### GET /types

List all registered entity types.

**Response:**
```json
{
  "types": [
    {
      "name": "file.media.image",
      "parent": "file.media",
      "description": "Image files"
    }
  ],
  "count": 25
}
```

---

### GET /types/{name}

Get details for a specific type.

**Response:**
```json
{
  "name": "file.media.image",
  "parent": "file.media",
  "ancestors": ["item", "file", "file.media"],
  "description": "Image files",
  "attributes": {...},
  "filters": {...}
}
```

---

## Filters

### GET /filters

Get available filters for search results.

**Query params:**
- `search` (string): Search query to extract filters from
- `type` (string): Entity type filter

**Response:**
```json
{
  "capabilities": {...},
  "filters": {
    "extensions": ["jpg", "png", "gif"],
    "mime_types": ["image/jpeg", "image/png"]
  }
}
```

---

## Providers

### GET /providers

List all registered providers.

**Response:**
```json
{
  "providers": [
    {
      "name": "filesystem",
      "connected": true,
      "supports_incremental": true
    }
  ],
  "count": 2
}
```

---

### GET /providers/status

Get detailed status of all providers.

**Response:**
```json
{
  "providers": [
    {
      "name": "filesystem",
      "connected": true,
      "last_discovery": "2024-01-01T00:00:00Z",
      "last_error": "",
      "entity_count": 1234,
      "supports_incremental": true
    }
  ],
  "count": 2
}
```

---

## Health

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "providers": {
    "total": 2,
    "connected": 2
  },
  "timestamp": 1703847600
}
```

---

### GET /

API index.

**Response:**
```json
{
  "name": "mifind API",
  "version": "0.1.0",
  "description": "Unified personal search API",
  "endpoints": {
    "/search": "POST - Search across all providers",
    "/search/federated": "POST - Search with per-provider results",
    "/entity/{id}": "GET - Get entity by ID",
    "/entity/{id}/expand": "GET - Get entity with relationships",
    "/entity/{id}/related": "GET - Get related entities",
    "/types": "GET - List all types",
    "/types/{name}": "GET - Get type details",
    "/filters": "GET - Get available filters",
    "/providers": "GET - List providers",
    "/providers/status": "GET - Provider status",
    "/health": "GET - Health check"
  }
}
```

---

## Entity ID Format

Entity IDs follow the format: `providerType:instanceID:entityID`

Examples:
- `filesystem:myfs:abc123` - File from filesystem provider
- `immich:photos:xyz789` - Photo from Immich provider
- `mock:default:test123` - Entity from mock provider

---

## Common Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | string | Filesystem path (files) |
| `extension` | string | File extension without dot |
| `mime_type` | string | MIME type |
| `size` | int | File size in bytes |
| `modified` | int | Modification time (unix timestamp) |
| `created` | int | Creation time (unix timestamp) |
| `width` | int | Image/video width |
| `height` | int | Image/video height |
| `duration` | int | Video/audio duration (seconds) |
