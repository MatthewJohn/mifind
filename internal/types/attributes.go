package types

import "time"

// Standard attribute keys that should be used across providers
// for common concepts. This enables unified filtering.
const (
	// Core identity attributes
	AttrID          = "id"
	AttrType        = "type"
	AttrProvider    = "provider"
	AttrTitle       = "title"
	AttrDescription = "description"

	// File/path attributes
	AttrPath      = "path"      // File system path or similar
	AttrSize      = "size"      // Size in bytes
	AttrExtension = "extension" // File extension
	AttrMimeType  = "mime_type" // MIME type
	AttrModified  = "modified"  // Last modification timestamp
	AttrCreated   = "created"   // Creation timestamp

	// Media attributes
	AttrDuration     = "duration" // Duration in seconds
	AttrWidth        = "width"    // Image/video width in pixels
	AttrHeight       = "height"   // Image/video height in pixels
	AttrCamera       = "camera"   // Camera make/model
	AttrLens         = "lens"     // Lens model
	AttrISO          = "iso"      // ISO setting
	AttrAperture     = "aperture" // Aperture f-number
	AttrShutterSpeed = "shutter"  // Shutter speed

	// GPS/location attributes
	AttrGPS       = "gps"       // GPS coordinates (lat, lng)
	AttrLatitude  = "latitude"  // Latitude coordinate
	AttrLongitude = "longitude" // Longitude coordinate
	AttrLocation  = "location"  // Human-readable location name

	// Media library attributes
	AttrAlbum  = "album"  // Album name
	AttrArtist = "artist" // Artist name
	AttrGenre  = "genre"  // Genre/category
	AttrYear   = "year"   // Release year
	AttrTrack  = "track"  // Track number

	// Version control attributes
	AttrRepository = "repository" // Repository name
	AttrBranch     = "branch"     // Branch name
	AttrCommit     = "commit"     // Commit hash
	AttrAuthor     = "author"     // Author name
	AttrFilePath   = "file_path"  // Path in repository

	// Task/issue attributes
	AttrStatus   = "status"   // Status (open, closed, etc.)
	AttrPriority = "priority" // Priority level
	AttrAssignee = "assignee" // Assigned user
	AttrLabels   = "labels"   // Labels/tags ([]string)
	AttrDueDate  = "due_date" // Due date

	// Face/person attributes (Immich, etc.)
	AttrFaces       = "faces"        // Number of faces detected
	AttrPeople      = "people"       // List of people IDs/names
	AttrFaceRegions = "face_regions" // Face detection regions

	// Smart search/ML attributes
	AttrSmartInfo = "smart_info" // ML-generated tags/info
	AttrScore     = "score"      // Relevance/confidence score

	// General attributes
	AttrName       = "name"        // Display name
	AttrBirthDate  = "birth_date"  // Birth date
	AttrIsFavorite = "is_favorite" // Favorite flag
	AttrIsArchived = "is_archived" // Archived flag
	AttrAssetCount = "asset_count" // Number of assets in collection
)

// GPS represents a geographic coordinate.
type GPS struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// AttributeBuilder helps build attribute maps with type safety.
type AttributeBuilder struct {
	attrs map[string]any
}

// NewAttributeBuilder creates a new AttributeBuilder.
func NewAttributeBuilder() *AttributeBuilder {
	return &AttributeBuilder{
		attrs: make(map[string]any),
	}
}

// Set sets a key-value pair.
func (b *AttributeBuilder) Set(key string, value any) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetString sets a string attribute.
func (b *AttributeBuilder) SetString(key, value string) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetInt sets an int attribute.
func (b *AttributeBuilder) SetInt(key string, value int) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetInt64 sets an int64 attribute.
func (b *AttributeBuilder) SetInt64(key string, value int64) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetFloat64 sets a float64 attribute.
func (b *AttributeBuilder) SetFloat64(key string, value float64) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetBool sets a bool attribute.
func (b *AttributeBuilder) SetBool(key string, value bool) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetStringSlice sets a string slice attribute.
func (b *AttributeBuilder) SetStringSlice(key string, value []string) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// SetGPS sets a GPS coordinate attribute.
func (b *AttributeBuilder) SetGPS(key string, lat, lng float64) *AttributeBuilder {
	b.attrs[key] = GPS{Latitude: lat, Longitude: lng}
	return b
}

// SetTime sets a time attribute as Unix timestamp.
func (b *AttributeBuilder) SetTime(key string, value int64) *AttributeBuilder {
	b.attrs[key] = value
	return b
}

// Build returns the constructed attributes map.
func (b *AttributeBuilder) Build() map[string]any {
	return b.attrs
}

// Merge merges another attribute map into this one.
// Existing keys are overwritten.
func (b *AttributeBuilder) Merge(other map[string]any) *AttributeBuilder {
	for k, v := range other {
		b.attrs[k] = v
	}
	return b
}

// Common attribute definitions for reuse across type definitions.
// These include full UI and Filter metadata to enable generic rendering.
var (
	// AttrDefPath is the standard path attribute definition.
	AttrDefPath = AttributeDef{
		Name:        AttrPath,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "File system path or resource identifier",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "input",
			Icon:     "Folder",
			Group:    "file",
			Label:    "Path",
			Priority: 10,
		},
		Filter: FilterConfig{
			SupportsEq:       true,
			SupportsContains: true,
			Cacheable:        false,
		},
	}

	// AttrDefSize is the standard size attribute definition.
	AttrDefSize = AttributeDef{
		Name:        AttrSize,
		Type:        AttributeTypeInt64,
		Required:    false,
		Filterable:  true,
		Description: "Size in bytes",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "range",
			Icon:     "HardDrive",
			Group:    "file",
			Label:    "Size",
			Priority: 11,
		},
		Filter: FilterConfig{
			SupportsRange: true,
			Cacheable:     false,
		},
	}

	// AttrDefModified is the standard modified time attribute definition.
	AttrDefModified = AttributeDef{
		Name:        AttrModified,
		Type:        AttributeTypeTime,
		Required:    false,
		Filterable:  true,
		Description: "Last modification timestamp (Unix)",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "date-range",
			Icon:     "Calendar",
			Group:    "metadata",
			Label:    "Modified",
			Priority: 20,
		},
		Filter: FilterConfig{
			SupportsRange: true,
			Cacheable:     false,
		},
	}

	// AttrDefCreated is the standard created time attribute definition.
	AttrDefCreated = AttributeDef{
		Name:        AttrCreated,
		Type:        AttributeTypeTime,
		Required:    false,
		Filterable:  true,
		Description: "Creation timestamp (Unix)",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "date-range",
			Icon:     "CalendarPlus",
			Group:    "metadata",
			Label:    "Created",
			Priority: 21,
		},
		Filter: FilterConfig{
			SupportsRange: true,
			Cacheable:     false,
		},
	}

	// AttrDefMimeType is the standard MIME type attribute definition.
	AttrDefMimeType = AttributeDef{
		Name:        AttrMimeType,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "MIME type of the resource",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "select",
			Icon:     "FileText",
			Group:    "file",
			Label:    "MIME Type",
			Priority: 12,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			SupportsNeq: true,
			Cacheable:   false,
		},
	}

	// AttrDefExtension is the standard extension attribute definition.
	AttrDefExtension = AttributeDef{
		Name:        AttrExtension,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "File extension without dot",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "select",
			Icon:     "File",
			Group:    "file",
			Label:    "Extension",
			Priority: 13,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			SupportsNeq: true,
			Cacheable:   false,
		},
	}

	// AttrDefDuration is the standard duration attribute definition.
	AttrDefDuration = AttributeDef{
		Name:        AttrDuration,
		Type:        AttributeTypeInt64,
		Required:    false,
		Filterable:  true,
		Description: "Duration in seconds",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "range",
			Icon:     "Clock",
			Group:    "media",
			Label:    "Duration",
			Priority: 30,
		},
		Filter: FilterConfig{
			SupportsRange: true,
			Cacheable:     false,
		},
	}

	// AttrDefWidth is the standard width attribute definition.
	AttrDefWidth = AttributeDef{
		Name:        AttrWidth,
		Type:        AttributeTypeInt,
		Required:    false,
		Filterable:  true,
		Description: "Image/video width in pixels",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "range",
			Icon:     "Maximize",
			Group:    "media",
			Label:    "Width",
			Priority: 31,
		},
		Filter: FilterConfig{
			SupportsRange: true,
			Cacheable:     false,
		},
	}

	// AttrDefHeight is the standard height attribute definition.
	AttrDefHeight = AttributeDef{
		Name:        AttrHeight,
		Type:        AttributeTypeInt,
		Required:    false,
		Filterable:  true,
		Description: "Image/video height in pixels",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "range",
			Icon:     "Maximize2",
			Group:    "media",
			Label:    "Height",
			Priority: 32,
		},
		Filter: FilterConfig{
			SupportsRange: true,
			Cacheable:     false,
		},
	}

	// AttrDefGPS is the standard GPS attribute definition.
	AttrDefGPS = AttributeDef{
		Name:        AttrGPS,
		Type:        AttributeTypeGPS,
		Required:    false,
		Filterable:  true,
		Description: "GPS coordinates (latitude, longitude)",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "gps",
			Icon:     "MapPin",
			Group:    "metadata",
			Label:    "GPS",
			Priority: 25,
		},
		Filter: FilterConfig{
			Cacheable: false,
		},
	}

	// AttrDefLocation is the standard location attribute definition.
	AttrDefLocation = AttributeDef{
		Name:        AttrLocation,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "Human-readable location name",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "input",
			Icon:     "Map",
			Group:    "metadata",
			Label:    "Location",
			Priority: 26,
		},
		Filter: FilterConfig{
			SupportsEq:       true,
			SupportsContains: true,
			Cacheable:        true,
			CacheTTL:         24 * time.Hour,
		},
	}

	// AttrDefCamera is the standard camera attribute definition.
	AttrDefCamera = AttributeDef{
		Name:        AttrCamera,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "Camera make/model",
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
		},
	}

	// AttrDefAlbum is the standard album attribute definition.
	AttrDefAlbum = AttributeDef{
		Name:        AttrAlbum,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "Album name",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "select",
			Icon:     "Album",
			Group:    "provider-immich",
			Label:    "Album",
			Priority: 40,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			SupportsNeq: true,
			Cacheable:   true,
			CacheTTL:    24 * time.Hour,
		},
	}

	// AttrDefArtist is the standard artist attribute definition.
	AttrDefArtist = AttributeDef{
		Name:        AttrArtist,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "Artist name",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "select",
			Icon:     "Music",
			Group:    "media",
			Label:    "Artist",
			Priority: 41,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			SupportsNeq: true,
			Cacheable:   true,
			CacheTTL:    24 * time.Hour,
		},
	}

	// AttrDefLabels is the standard labels attribute definition.
	AttrDefLabels = AttributeDef{
		Name:        AttrLabels,
		Type:        AttributeTypeStringSlice,
		Required:    false,
		Filterable:  true,
		Description: "Labels or tags",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "multiselect",
			Icon:     "Tag",
			Group:    "metadata",
			Label:    "Labels",
			Priority: 50,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			Cacheable:   true,
			CacheTTL:    24 * time.Hour,
		},
	}

	// AttrDefStatus is the standard status attribute definition.
	AttrDefStatus = AttributeDef{
		Name:        AttrStatus,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "Status of the item",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "select",
			Icon:     "Badge",
			Group:    "metadata",
			Label:    "Status",
			Priority: 60,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			SupportsNeq: true,
			Cacheable:   false,
		},
	}

	// AttrDefPriority is the standard priority attribute definition.
	AttrDefPriority = AttributeDef{
		Name:        AttrPriority,
		Type:        AttributeTypeString,
		Required:    false,
		Filterable:  true,
		Description: "Priority level",
		AlwaysVisible: false,
		UI: UIConfig{
			Widget:   "select",
			Icon:     "Flag",
			Group:    "metadata",
			Label:    "Priority",
			Priority: 61,
		},
		Filter: FilterConfig{
			SupportsEq:  true,
			SupportsNeq: true,
			Cacheable:   false,
		},
	}
)
