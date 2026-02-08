# mifind

**mifind** is a unified personal search tool.

It lets you search *your* data — across many systems — as if it all lived in one place.

Code, documents, media, files, metadata, and more. One search.

---

## What is mifind?

Most personal data lives in silos:

- GitLab for code and issues  
- Jellyfin for movies and TV shows  
- Photo libraries for images and videos  
- Filesystems full of documents  
- Infrastructure tools with their own UIs  

Each system is good at what it does — but terrible at **working together**.

**mifind** sits above them.

It builds a unified index of *lightweight representations* of your data and lets you:
- Search everything at once
- Filter dynamically by type and attributes
- Explore relationships between items
- Retrieve full details on demand from the original source

The original systems remain the source of truth.  
mifind just helps you *find* things.

---

## Core ideas

### Unified search
Search across multiple personal systems with a single query.

### Typed entities
Everything in mifind has a type, with a strong hierarchy. Example:

```text
Item
└── File
    ├── MediaFile
    │   ├── MusicFile
    │   └── VideoFile
    └── DocumentFile
Item
└── Media
    ├── Movie
    └── Episode
```

Types define which attributes and filters make sense.

### Adaptive filters
Search behaves like modern marketplaces:
1. Start broad
2. See result categories and counts
3. Narrow by type
4. Apply filters relevant to that type

Filters are dynamic — not hardcoded.

### Relationships
Some items are connected:
- A movie ↔ the file on disk
- A song ↔ its album ↔ its artist
- A GitLab issue ↔ a repository ↔ a file

mifind understands and exposes these relationships so you can navigate context, not just lists.

### Pluggable providers
Each data source is integrated via a provider:
- Providers discover items
- Map external data to internal types
- Expose relationships
- Hydrate full data on demand

New providers and new item types can be added in code.

---

## Caching (an implementation detail)

mifind uses a lightweight cache to make search fast:
- Item summaries
- Search indexes
- Faceted aggregations

The cache is **not** a source of truth.  
It can be pruned, rebuilt, or discarded at any time.

All authoritative data lives in the providers.

---

## AI & agent-friendly by design

mifind exposes a structured, entity-centric API that works well for AI agents.

Instead of blobs of text, agents can:
- Search for entities
- Inspect types and attributes
- Traverse relationships
- Fetch full details only when needed

This makes mifind a natural backend for:
- MCP servers
- Personal assistants
- Tool-using agents

Symbolic search first. Semantic reasoning on top.

---

## What mifind is not

- ❌ A replacement for your existing tools
- ❌ A document store or media library
- ❌ A sync engine
- ❌ A data hoarder

mifind helps you *find* things — it doesn't own them.

---

## Running

### Setup

```bash
# Copy example configs (first time or when updating)
cp config/examples/mifind.yaml config/
cp config/examples/filesystem-api.yaml config/

# Edit configs to set your paths, ports, etc.
vim config/mifind.yaml
vim config/filesystem-api.yaml
```

### mifind API

```bash
go run cmd/mifind/main.go
```

### filesystem-api

```bash
# 1. Start Meilisearch
docker run -p 7700:7700 getmeili/meilisearch

# 2. Configure scan paths in config/filesystem-api.yaml

# 3. Start filesystem-api
go run cmd/filesystem-api/main.go
```

Environment variables:

| Service | Prefix | Example |
|---------|--------|---------|
| mifind | `MIFIND_` | `MIFIND_HTTP_PORT=8080` |
| filesystem-api | `MIFIND_FILESYSTEM_` | `MIFIND_FILESYSTEM_MEILISEARCH_URL=http://localhost:7700` |

**API Docs:** See [docs/API.md](docs/API.md) for complete API reference.

---

## Status

Early-stage / experimental.

The focus is currently on:
- Core data model
- Provider abstractions
- Search and filtering behavior
- API shape (human + agent use)

Expect breaking changes.
