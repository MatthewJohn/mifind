# Personal Data Unified Search Tool

## **Summary**
Build a **unified personal data search tool** that allows searching across multiple personal systems (GitLab, Jellyfin, personal files, media servers, etc.) from a single interface. The tool supports **full-text search, adaptive filters, and entity relationships**, enabling humans and AI agents to query personal data efficiently.

A **lightweight cache** will be used to speed up search and faceted aggregations, but the cache is an **optimization**, not the primary functionality. The source of truth remains the providers themselves.

---

## **Goals / Requirements**

### Functional
- Search across multiple personal data sources in one query
- Support hierarchical item types (e.g., `File -> MusicFile`, `Media -> Movie`)
- Expose **adaptive filters** per type, similar to eBay-style faceted search
- Maintain **relationships** between items (e.g., `Song -> Album -> Artist`, `Movie -> File`)
- Providers are **pluggable** and configurable; new providers and types can be added
- Enable **AI agent integration** via structured entity API (e.g., MCP server)
- Use a **cache layer** to improve search speed and faceting performance

### Non-functional
- Lightweight and efficient indexing; do not store full item payloads
- Cache layer supports pruning and eviction:
  - TTL per item
  - Provider-based flush
  - Type-based purge
  - Optional LRU/MRU
- Fast full-text search + faceted filtering
- Incremental updates; avoid full reindexing
- Separation of search/index layer from provider hydration

---

## **High-Level Design**

### Architecture Overview

```
    +--------------------+       +--------------------+
    |  Provider Layer    |       |  Optional Persistence|
    | (GitLab, Jellyfin, |       | (SQLite/Tantivy snapshot)
    |  FileSystem, etc.) |       +--------------------+
    +--------------------+
              |
              v
    +--------------------+
    |  Unified Search    | <- core search engine
    |  Tool              |
    | - Item summaries   |
    | - Attributes       |
    | - Relationships    |
    | - Full-text index  |
    | - Faceted aggregations
    +--------------------+
              |
              v
    +--------------------+
    |  Cache Layer       | <- lightweight cache to speed up searches
    | - Precomputed aggregates
    | - Frequently accessed items
    +--------------------+
              |
              v
    +--------------------+
    |  API / MCP Server  | <- structured endpoints for human/AI queries
    | - search_entities()|
    | - describe_entity()|
    | - expand_entity()  |
    +--------------------+

```


---

### Core Concepts
- **Item**: Lightweight representation of a personal object
  - `id` (stable, provider-scoped)
  - `type` (hierarchical)
  - `attributes` (key-value for filtering)
  - `relationships` (optional, lazy)
  - `search_tokens` (flattened text for search)
- **Type System**
  - Hierarchical: e.g., `Item -> File -> MediaFile -> MusicFile`
  - Defines filters and valid attributes, independent of provider
- **Provider**
  - Discovers items and maps them to internal types
  - Provides stable IDs
  - Hydrates full data on-demand
  - Supports incremental updates
- **Cache Layer**
  - Stores precomputed metadata, aggregates, and frequently accessed summaries
  - Optimizes search speed and faceted filtering
  - Eviction strategies configurable (TTL, type, provider, LRU)

---

### Search Model

#### Full-text Search
- Search across all indexed item summaries
- Tokenization & normalization across fields:
  - `title/name`
  - `tags`
  - flattened attributes
- Supports partial matches, stemming, synonyms

#### Two-phase / Adaptive Search
1. Broad, type-agnostic search across all items
2. Narrowed, type-aware filtering with dynamic facets

- Backend exposes:
  - `available_filters(result_set) -> {filter_name: values/counts}`
  - `apply_filters(filters) -> updated_result_set`
- Filters respect type hierarchy and inheritance

#### Relationship-aware Search
- Relationships influence search when requested
- Backend supports:
  - `get_related(entity_id)`  
  - Filtering by relationship type

#### Adaptive / Contextual Results
- Type counts like “categories” in search results
- Drill-down flow:
  1. Broad search → see types and counts
  2. Select type → update filters to relevant attributes
  3. Apply filters → update results dynamically

#### Optional Semantic Search
- Embeddings for summaries for similarity-based rerank
- Hybrid search: symbolic (structured) first, semantic second

---

### Frontend / Dynamic Search Requirements
- **Dynamic filter UI**: filters depend on current search query and result type
  - Multi-select, range sliders, enums/checkboxes
  - Toggle relationship-based filters
- **Type drill-down**: show categories with counts; selecting a type updates filters
- **Relationship exploration**: expand items to see related entities; update filters dynamically
- **Pagination / lazy loading**: incremental rendering, on-demand hydration
- **Search-as-you-type**: typeahead suggestions, dynamic type counts
- **AI / agent integration hooks**: structured JSON output, optional semantic hints

---

### Backend ↔ Frontend Interaction

| Action | Backend | Frontend |
|--------|---------|----------|
| User types query | Tokenize, match, return IDs + counts | Show suggestions & type counts |
| User selects type | Filter result set, recompute dynamic filters | Update filter UI to match type |
| User applies filters | Aggregate remaining items for counts | Update filters UI and highlight selections |
| User expands entity | Return relationships + summaries | Show preview, allow further drill-down |
| Pagination / lazy loading | Slice result set | Incrementally render items |
| Agent requests entity | Return structured summary & relationships | Agent consumes JSON directly |

---

## **Design Decisions**
- **Datastore**: In-memory search index (Tantivy/Meilisearch) + lightweight SQLite/Redis for metadata/relationships
- **Unified search tool**: core system integrates providers, handles search, relationships, and adaptive filtering
- **Cache layer**: optional optimization for aggregates and frequently accessed summaries
- **Providers**: pluggable, configurable, incremental updates
- **Search-first philosophy**: symbolic index + optional semantic layer
- **Frontend driven by backend metadata**: filters & type categories dynamically generated

---

## **Open Questions / Things to Investigate**
1. Optimal cache design: TTL vs LRU vs hybrid strategies
2. Optimal search engine for personal-scale unified search: Tantivy vs Meilisearch vs Redis/RediSearch
3. Schema evolution: adding new types, attributes, relationships safely
4. Relationship storage design for fast traversal, low memory
5. MCP / AI integration:
   - Minimal API design for search & expand
   - Semantic ranking for top-K results
6. Incremental indexing and provider change detection strategies
7. Optional embedding layer: storage, size, eviction for semantic search
8. Frontend UX patterns for dynamic filters, type drill-down, relationship previews
9. Hydration patterns: lazy vs prefetch for performance

