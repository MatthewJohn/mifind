# Attribute System Guide

This guide explains how to add attributes to mifind, including filtering behavior, UI configuration, and provider extensions.

## Overview

The mifind attribute system is designed to be **generic and metadata-driven**. Core attribute definitions live in `internal/types/`, and providers can extend/override these definitions through `AttributeExtensions()`. The frontend uses this metadata to render filters generically without hardcoded knowledge.

## Key Concepts

### Attribute Definition Locations

1. **Core Attributes** (`internal/types/attributes.go`): Base definitions for common attributes used across providers
2. **Provider Extensions** (`pkg/provider/<name>/provider.go`): Provider-specific overrides that extend core definitions
3. **Type Attributes** (`internal/types/<type>.go`): Type-specific attributes (e.g., media-specific attributes)

### Attribute Metadata Structure

```go
type AttributeDef struct {
    Name          string      // Attribute key name
    Type          AttributeType // Data type (string, int, bool, time, gps, []string)
    Required      bool        // Must be present on entities
    Filterable    bool        // Can be used for filtering
    Description   string      // Human-readable description
    AlwaysVisible bool        // Always show filter even without results
    UI            UIConfig    // How to display in frontend
    Filter        FilterConfig // How filtering works
}
```

## Decision Guide: Adding a New Attribute

When adding a new attribute, make these decisions in order:

### 1. Core or Provider-Specific?

**Core Attribute** - Use when:
- The concept is common across multiple providers (e.g., camera, width, height, location)
- The attribute has a standard meaning that applies broadly
- Multiple providers might support it

**Provider-Specific** - Use when:
- The attribute is unique to one provider (e.g., Immich's "smart info")
- The semantics are provider-specific even if the name overlaps

**Example:**
- ✅ Core: `camera`, `width`, `height`, `location.city` - common concepts
- ✅ Provider-specific: Immich's `smart_info` (ML tags specific to Immich)

### 2. What is the Data Type?

Choose the appropriate `AttributeType`:

| Type | Use Case | Example |
|------|----------|---------|
| `string` | Text values | Camera make, album name |
| `int` | Numeric values | Width, height |
| `int64` | Large numbers | Unix timestamps, file sizes |
| `float64` | Decimal numbers | GPS coordinates |
| `bool` | Yes/No values | Is favorite, is archived |
| `time` | Timestamps | Created date, modified date |
| `gps` | Coordinates | GPS location |
| `[]string` | Multiple values | Tags, labels, people |

### 3. How Should Filtering Work?

Configure the `FilterConfig`:

```go
type FilterConfig struct {
    SupportsEq       bool              // Equality: "field=value"
    SupportsNeq      bool              // Inequality: "field!=value"
    SupportsRange    bool              // Range: "field>value", "field<value"
    SupportsContains bool              // Substring: "field~value"
    Cacheable        bool              // Can filter values be cached?
    CacheTTL         time.Duration     // How long to cache
    ProviderLevel    bool              // Filtered by provider API, not entity attributes
    ValueSource      FilterValueSource // Where filter values come from
    ShowZeroCount    bool              // Show options with 0 count in results
}
```

#### Critical Decision: ValueSource

The `ValueSource` field determines where filter values come from:

**`FilterValueFromEntities`** (default)
- Values extracted from **current search results ONLY**
- No provider pre-fetch
- Use for: Attributes where values depend entirely on search context
- Examples: `extension`, `size`, `mime_type` (only show what's in results)
- Behavior: Shows only values present in current paginated results
- NOT cached (varies by search)

**`FilterValueFromProvider`**
- Values from **provider's complete list** (cached)
- Counts from provider totals, NOT contextual to search
- Use for: Static lists where counts don't change based on search
- Examples: `person`, `album` (provider has complete list, counts don't vary)
- Behavior: Shows all provider options with provider's total counts
- Cached for 24h

**`FilterValueHybrid`**
- Values from **provider's complete list** (cached)
- Counts from **current search results** (contextual)
- Use for: Provider-based lists where you want all options but contextual counts
- Examples: `location.city`, `location.state`, `location.country`
- Behavior: Shows all provider options, counts update based on search (0 if not in results)
- Cached for 24h

**Decision Tree:**
```
Does the provider have a complete, enumerable list of all possible values?
├─ Yes → Provider-based (which one?)
│  ├─ Do counts vary based on search context?
│  │  ├─ Yes → FilterValueHybrid (provider list + entity counts)
│  │  └─ No → FilterValueFromProvider (provider list + provider totals)
│  └─ Examples: cities (Hybrid), people/albums (FromProvider)
└─ No → FilterValueFromEntities
   └─ Values extracted from current search results
   └─ Examples: extension, size, mime_type
```

**Behavior Comparison Table:**

| Attribute Type | List Source | Count Source | Cached? | Shows 0-count options? |
|----------------|-------------|--------------|---------|----------------------|
| FromEntities | Search results | Search results | No | Only if in results |
| FromProvider | Provider API | Provider totals | Yes (24h) | Always (all options) |
| Hybrid | Provider API | Search results | Yes (24h) | If ShowZeroCount=true |

#### ShowZeroCount

When `ValueSource` is `FilterValueHybrid`:
- `ShowZeroCount: true` - Show all provider options even if count is 0 in results (e.g., all cities)
- `ShowZeroCount: false` - Hide options with 0 count (only show cities present in results)

When `ValueSource` is `FilterValueFromEntities` or `FilterValueFromProvider`:
- Has no effect (FromEntities only shows what's in results, FromProvider always shows all)

**Rule of thumb:**
- **Geographic/Reference data** (cities, countries, states): `ValueSource: Hybrid` + `ShowZeroCount: true`
- **User-generated collections** (albums, playlists): `ValueSource: FromProvider` + `ShowZeroCount: false`
- **Result-dependent attributes** (extensions, sizes): `ValueSource: FromEntities` + `ShowZeroCount: false`

#### ProviderLevel

Set `ProviderLevel: true` when:
- The filter is handled by provider API calls, **not** stored as entity attributes
- Examples: Immich `person`, `album`, `location.city` filters
- The provider does the filtering server-side

**Important:** Provider-level filters are excluded from entity-level attribute filtering in the ranker.

### 4. What UI Widget Should Be Used?

Configure the `UIConfig`:

```go
type UIConfig struct {
    Widget   string // UI widget type
    Icon     string // Icon name (lucide-react)
    Group    string // Display group for organization
    Label    string // Display label
    Priority int    // Display order (lower = first)
}
```

**Widget Options:**

| Widget | Use Case | Multi-Select? |
|--------|----------|---------------|
| `input` | Free-text search | No |
| `select` | Dropdown from predefined options | No |
| `multiselect` | Multiple selection | Yes |
| `checkbox-group` | Multiple choice, show all | Yes |
| `date-range` | Date range picker | No |
| `range` | Numeric range (min/max) | No |
| `bool` | Yes/No toggle | No |
| `gps` | GPS coordinates | No |

**Widget Decision Guide:**

```
Is the value set enumerable (known finite options)?
├─ Yes → Use select/multiselect/checkbox-group
│  │
│  └─ Should user select multiple values?
│     ├─ Yes → multiselect or checkbox-group
│  │   │  └─ checkbox-group if always show all options
│  │   └─ multiselect if searchable/dropdown
│  └─ No → select
│
└─ No (free-text or range)
   ├─ Numeric range? → range
   ├─ Date range? → date-range
   ├─ Yes/No? → bool
   └─ Text search? → input
```

**Icon Names:** Use lucide-react icon names (e.g., "Users", "Map", "Camera", "File")

**Group Names:** Organize filters by category:
- `core` - Core entity attributes (type, id)
- `file` - File attributes (path, extension, size)
- `media` - Media attributes (width, height, camera, duration)
- `metadata` - Metadata (created, modified, gps, location)
- `provider-<name>` - Provider-specific (e.g., "provider-immich")

### 5. Should It Be Always Visible?

Set `AlwaysVisible: true` when:
- The filter is important enough to show even with no search results
- Examples: `type` (entity type), `person` (people in photos)

**When to use:**
- Primary navigation filters (entity type)
- High-value discovery filters (people, albums)
- **Don't overuse** - too many always-visible filters clutters the UI

## Implementation Examples

### Example 1: Simple Core Attribute (Camera)

```go
// In internal/types/attributes.go

var AttrDefCamera = AttributeDef{
    Name:          AttrCamera,
    Type:          AttributeTypeString,
    Required:      false,
    Filterable:    true,
    Description:   "Camera make/model",
    AlwaysVisible: false,
    UI: UIConfig{
        Widget:   "select",
        Icon:     "Camera",
        Group:    "media",
        Label:    "Camera",
        Priority: 33,
    },
    Filter: FilterConfig{
        SupportsEq:  true,
        SupportsNeq: true,
        Cacheable:   true,
        CacheTTL:    24 * time.Hour,
        // ValueSource defaults to FilterValueResultBased
        // Values extracted from search results
    },
}
```

**Decisions made:**
- ✅ Core attribute - common across photo providers
- ✅ String type - text value
- ✅ Select widget - enumerable (cameras present in results)
- ✅ Result-based - only show cameras in current results
- ✅ Cacheable - camera list doesn't change often
- ❌ Not always visible - secondary filter

### Example 2: Provider-Based Location Filter (City)

```go
// In internal/types/attributes.go (core definition)

var AttrDefLocationCity = AttributeDef{
    Name:          AttrLocationCity,
    Type:          AttributeTypeString,
    Required:      false,
    Filterable:    true,
    Description:   "City name",
    AlwaysVisible: false,
    UI: UIConfig{
        Widget:   "select",
        Icon:     "Map",
        Group:    "metadata",
        Label:    "City",
        Priority: 28,
    },
    Filter: FilterConfig{
        SupportsEq:       true,
        SupportsNeq:      true,
        SupportsContains: true,
        Cacheable:        true,
        CacheTTL:         24 * time.Hour,
        ValueSource:      FilterValueHybrid,  // Provider list + entity counts
        ShowZeroCount:    true,
    },
}
```

```go
// In pkg/provider/immich/provider.go (provider extension)

func (p *Provider) AttributeExtensions(ctx context.Context) map[string]types.AttributeDef {
    return map[string]types.AttributeDef{
        types.AttrLocationCity: {
            // Override core with provider-specific settings
            Name:       types.AttrLocationCity,
            Type:       types.AttributeTypeString,
            Filterable: true,
            Description: "City name - filter handled by Immich API",
            UI: types.UIConfig{
                Widget: "select",  // Must match core!
                Icon:   "MapPin",
                Group:  "provider-immich",
                Label:  "City",
                Priority: 12,
            },
            Filter: types.FilterConfig{
                SupportsEq:       true,
                SupportsNeq:      true,
                SupportsContains: true,
                Cacheable:        true,
                CacheTTL:         24 * time.Hour,
                ProviderLevel:    true,  // Filtered by Immich API
                ValueSource:      types.FilterValueHybrid,  // Provider list + entity counts
                ShowZeroCount:    true,  // Must match core!
            },
        },
    }
}
```

**Decisions made:**
- ✅ Core definition exists - common concept
- ✅ Provider extension adds `ProviderLevel: true` - Immich handles filtering
- ✅ Hybrid value source - Provider list + contextual entity counts
- ✅ ShowZeroCount - show all cities even if none in current results
- ✅ Select widget - single location selection
- ✅ Cacheable - city list changes infrequently

### Example 3: Multi-Select Person Filter

```go
// In internal/types/attributes.go

var AttrDefPerson = AttributeDef{
    Name:          AttrPerson,
    Type:          AttributeTypeStringSlice,  // Multiple values
    Required:      false,
    Filterable:    true,
    Description:   "People detected in media (faces)",
    AlwaysVisible: true,  // Always show - high-value filter
    UI: UIConfig{
        Widget:   "multiselect",
        Icon:     "Users",
        Group:    "media",
        Label:    "People",
        Priority: 41,
    },
    Filter: FilterConfig{
        SupportsEq:    true,
        Cacheable:     true,
        CacheTTL:      24 * time.Hour,
        ProviderLevel: true,  // Filtered by provider API
        ValueSource:   FilterValueFromProvider,  // Provider list with provider totals
        ShowZeroCount: true,
    },
}
```

**Decisions made:**
- ✅ StringSlice type - multiple people per photo
- ✅ Multiselect widget - can select multiple people
- ✅ Always visible - important discovery filter
- ✅ FromProvider value source - Provider list with provider totals (not contextual)
- ✅ Provider-level - provider API handles filtering

### Example 4: Result-Based Extension Filter

```go
// In internal/types/attributes.go

var AttrDefExtension = AttributeDef{
    Name:          AttrExtension,
    Type:          AttributeTypeString,
    Required:      false,
    Filterable:    true,
    Description:   "File extension without dot",
    AlwaysVisible: false,
    UI: UIConfig{
        Widget:   "select",
        Icon:     "File",
        Group:    "file",
        Label:    "Extension",
        Priority: 13,
    },
    Filter: FilterConfig{
        SupportsEq:    true,
        SupportsNeq:   true,
        Cacheable:     false,  // Don't cache - varies by search
        ValueSource:   types.FilterValueFromEntities,  // Extract from results only
        ShowZeroCount: false,  // Hide unused extensions
    },
}
```

**Decisions made:**
- ✅ FromEntities value source - only show extensions in current results
- ✅ Not cacheable - varies by search context
- ✅ ShowZeroCount false - don't show extensions with 0 results
- ✅ Select widget - single extension selection

## Provider Extension Best Practices

### When to Use AttributeExtensions

Use `AttributeExtensions()` to:

1. **Add ProviderLevel flag** - When provider handles filtering server-side
2. **Override UI for provider context** - Different grouping/priority
3. **Add provider-specific attributes** - Unique to that provider

### What NOT to Override

**Don't override core metadata unless necessary:**
- Widget type - should be consistent across providers
- ValueSource - determines fundamental behavior
- ShowZeroCount - affects UX consistently

**Exception:** When provider has fundamentally different behavior (e.g., Immich's location filtering vs filesystem's)

### Extension Pattern

```go
func (p *Provider) AttributeExtensions(ctx context.Context) map[string]types.AttributeDef {
    return map[string]types.AttributeDef{
        // Pattern: Only override what's different
        types.AttrPerson: {
            // Copy core structure
            Name:       types.AttrPerson,
            Type:       types.AttributeTypeStringSlice,
            Filterable: true,

            // Provider-specific overrides
            Description: "People detected in media (faces) - Immich face recognition",
            AlwaysVisible: true,  // Keep core setting

            UI: types.UIConfig{
                Widget: "multiselect",  // Must match core
                Icon:   "Users",
                Group:  "provider-immich",  // Provider-specific grouping
                Label:  "People",
                Priority: 10,  // Higher priority in Immich context
            },
            Filter: types.FilterConfig{
                SupportsEq:     true,
                Cacheable:      true,
                CacheTTL:       24 * time.Hour,
                ProviderLevel:  true,  // KEY: Add provider-level flag
                // ValueSource inherited from core
            },
        },

        // Provider-specific attribute (not in core)
        "immich_smart_info": {
            Name:       "smart_info",
            Type:       types.AttributeTypeString,
            Filterable: true,
            Description: "ML-generated smart search tags",
            UI: types.UIConfig{
                Widget: "input",
                Icon:   "Sparkles",
                Group:  "provider-immich",
                Label:  "Smart Tags",
            },
            Filter: types.FilterConfig{
                SupportsContains: true,
            },
        },
    }
}
```

## Common Pitfalls

### Pitfall 1: Inconsistent ValueSource

❌ **Wrong:** Core has `ValueSource: ProviderBased` but provider extension omits it
✅ **Right:** Both core and extension agree on ValueSource

### Pitfall 2: Widget Mismatch

❌ **Wrong:** Core has `Widget: "select"` but extension uses `"input"`
✅ **Right:** Extension matches core or explicitly documents the difference

### Pitfall 3: Missing JSON Tags

❌ **Wrong:** Go struct without JSON tags
```go
type FilterOption struct {
    Value string
    Label string
}
```

✅ **Right:** Include JSON tags for API serialization
```go
type FilterOption struct {
    Value string `json:"value"`
    Label string `json:"label"`
}
```

### Pitfall 4: Forgetting ProviderLevel

❌ **Wrong:** Provider API filter but `ProviderLevel: false`
- Result: Ranker tries to filter by entity attributes (which don't exist)

✅ **Right:** Set `ProviderLevel: true` for provider-filtered attributes

### Pitfall 5: Cacheable Without ValueSource

❌ **Wrong:** `Cacheable: true` with `ValueSource: FromEntities`
- Result: Caches values that vary by search query

✅ **Right:** Only cache when using `ValueSource: FromProvider` or `ValueSource: Hybrid`

## Testing Checklist

After adding an attribute:

- [ ] Backend: Attribute appears in `/api/filters` response
- [ ] Backend: `attributes[<name>]` has correct metadata
- [ ] Backend: `values[<name>]` populated if ProviderBased
- [ ] Frontend: Filter appears in UI
- [ ] Frontend: Correct widget type (select/input/etc.)
- [ ] Frontend: Options populated if ProviderBased
- [ ] Functional: Applying filter works correctly
- [ ] Functional: Filter values update after search

## Quick Reference

### Attribute Constants

Define attribute name constants in `internal/types/attributes.go`:
```go
const (
    AttrMyAttribute = "my_attribute"
)
```

### Attribute Definition Template

```go
var AttrDefMyAttribute = AttributeDef{
    Name:          AttrMyAttribute,
    Type:          AttributeTypeString,
    Required:      false,
    Filterable:    true,
    Description:   "Human-readable description",
    AlwaysVisible: false,
    UI: UIConfig{
        Widget:   "select",  // or "input", "multiselect", etc.
        Icon:     "IconName",
        Group:    "group-name",
        Label:    "Display Label",
        Priority: 50,
    },
    Filter: FilterConfig{
        SupportsEq:       true,
        SupportsNeq:      false,
        SupportsRange:    false,
        SupportsContains: false,
        Cacheable:        false,
        CacheTTL:         0,
        ProviderLevel:    false,
        ValueSource:      FilterValueFromEntities,  // Choose: FromEntities, FromProvider, Hybrid
        ShowZeroCount:    false,
    },
}
```

### Register with Core Attributes

Add to `CoreAttributes` map in `internal/types/core.go`:
```go
var CoreAttributes = map[string]AttributeDef{
    types.AttrType:    TypeAttribute,
    types.AttrMyAttribute: AttrDefMyAttribute,  // Add here
    // ... other core attributes
}
```
