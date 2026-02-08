package types

// Core entity type definitions.
// These are the canonical type names used throughout mifind.
// Providers should map their internal types to these constants.

const (
	// Root types
	TypeItem       = "item"
	TypeMedia      = "media"
	TypeCollection = "collection"
	TypePerson     = "person"

	// File types
	TypeFile              = "file"
	TypeFileMedia         = "file.media"
	TypeFileMediaVideo    = "file.media.video"
	TypeFileMediaImage    = "file.media.image"
	TypeFileMediaMusic    = "file.media.music"
	TypeFileDocument      = "file.document"
	TypeFileDocumentPDF   = "file.document.pdf"
	TypeFileDocumentWord  = "file.document.word"
	TypeFileDocumentSheet = "file.document.spreadsheet"
	TypeFileDocumentPres  = "file.document.presentation"
	TypeFileDocumentText  = "file.document.text"
	TypeFileDocumentHTML  = "file.document.html"
	TypeFileDocumentCode  = "file.document.code"
	TypeFileArchive       = "file.archive"

	// Media asset types (from servers like Immich)
	TypeMediaAsset      = "media.asset"
	TypeMediaAssetPhoto = "media.asset.photo"
	TypeMediaAssetVideo = "media.asset.video"

	// Collection types
	TypeCollectionAlbum    = "collection.album"
	TypeCollectionFolder   = "collection.folder"
	TypeCollectionPlaylist = "collection.playlist"
)

// TypeHierarchy defines the parent-child relationships for all types.
// This can be used to build the type registry programmatically.
var TypeHierarchy = []TypeDefinition{
	// Root types
	{
		Name:        TypeItem,
		Description: "Base type for all items",
		Attributes:  make(map[string]AttributeDef),
		Filters:     []FilterDefinition{},
	},
	{
		Name:        TypeMedia,
		Description: "Base type for media items",
		Parent:      TypeItem,
		Attributes:  make(map[string]AttributeDef),
		Filters:     []FilterDefinition{},
	},
	{
		Name:        TypeCollection,
		Description: "Base type for collections",
		Parent:      TypeItem,
		Attributes:  make(map[string]AttributeDef),
		Filters:     []FilterDefinition{},
	},
	{
		Name:        TypePerson,
		Description: "Person or detected face",
		Parent:      TypeItem,
		Attributes: map[string]AttributeDef{
			AttrName:      {Name: AttrName, Type: AttributeTypeString, Filterable: true},
			AttrBirthDate: {Name: "birth_date", Type: AttributeTypeString, Filterable: true},
		},
		Filters: []FilterDefinition{
			{Name: "is_hidden", Type: FilterTypeBool, Label: "Hidden"},
		},
	},

	// File types
	{
		Name:   TypeFile,
		Parent: TypeItem,
		Attributes: map[string]AttributeDef{
			AttrPath:      {Name: AttrPath, Type: AttributeTypeString, Filterable: true},
			AttrSize:      {Name: AttrSize, Type: AttributeTypeInt64, Filterable: true},
			AttrExtension: {Name: AttrExtension, Type: AttributeTypeString, Filterable: true},
			AttrMimeType:  {Name: AttrMimeType, Type: AttributeTypeString, Filterable: true},
			AttrModified:  {Name: AttrModified, Type: AttributeTypeTime, Filterable: true},
			AttrCreated:   {Name: AttrCreated, Type: AttributeTypeTime, Filterable: true},
		},
		Filters: []FilterDefinition{
			{Name: AttrExtension, Type: FilterTypeSelect, Label: "Extension"},
			{Name: AttrMimeType, Type: FilterTypeSelect, Label: "MIME Type"},
		},
	},
	{
		Name:        TypeFileMedia,
		Parent:      TypeFile,
		Description: "Media file (audio, video, image)",
	},
	{
		Name:   TypeFileMediaVideo,
		Parent: TypeFileMedia,
		Attributes: map[string]AttributeDef{
			AttrDuration: {Name: AttrDuration, Type: AttributeTypeInt64, Filterable: true},
			AttrWidth:    {Name: AttrWidth, Type: AttributeTypeInt, Filterable: true},
			AttrHeight:   {Name: AttrHeight, Type: AttributeTypeInt, Filterable: true},
		},
	},
	{
		Name:   TypeFileMediaImage,
		Parent: TypeFileMedia,
		Attributes: map[string]AttributeDef{
			AttrWidth:  {Name: AttrWidth, Type: AttributeTypeInt, Filterable: true},
			AttrHeight: {Name: AttrHeight, Type: AttributeTypeInt, Filterable: true},
			AttrCamera: {Name: AttrCamera, Type: AttributeTypeString, Filterable: true},
			AttrGPS:    {Name: AttrGPS, Type: AttributeTypeGPS, Filterable: true},
		},
	},
	{
		Name:   TypeFileMediaMusic,
		Parent: TypeFileMedia,
		Attributes: map[string]AttributeDef{
			AttrDuration: {Name: AttrDuration, Type: AttributeTypeInt64, Filterable: true},
			AttrAlbum:    {Name: AttrAlbum, Type: AttributeTypeString, Filterable: true},
			AttrArtist:   {Name: AttrArtist, Type: AttributeTypeString, Filterable: true},
			AttrGenre:    {Name: AttrGenre, Type: AttributeTypeString, Filterable: true},
		},
	},
	{
		Name:        TypeFileDocument,
		Parent:      TypeFile,
		Description: "Document file",
	},

	// Media asset types (from servers like Immich, Jellyfin)
	{
		Name:        TypeMediaAsset,
		Parent:      TypeMedia,
		Description: "Media asset from a media server",
		Attributes: map[string]AttributeDef{
			AttrPath:       {Name: AttrPath, Type: AttributeTypeString, Filterable: true},
			AttrSize:       {Name: AttrSize, Type: AttributeTypeInt64, Filterable: true},
			AttrModified:   {Name: AttrModified, Type: AttributeTypeTime, Filterable: true},
			AttrCreated:    {Name: AttrCreated, Type: AttributeTypeTime, Filterable: true},
			AttrIsFavorite: {Name: "is_favorite", Type: AttributeTypeBool, Filterable: true},
			AttrIsArchived: {Name: "is_archived", Type: AttributeTypeBool, Filterable: true},
		},
		Filters: []FilterDefinition{
			{Name: "is_favorite", Type: FilterTypeBool, Label: "Favorite"},
			{Name: "is_archived", Type: FilterTypeBool, Label: "Archived"},
		},
	},
	{
		Name:        TypeMediaAssetPhoto,
		Parent:      TypeMediaAsset,
		Description: "Photo asset",
		Attributes: map[string]AttributeDef{
			AttrWidth:    {Name: AttrWidth, Type: AttributeTypeInt, Filterable: true},
			AttrHeight:   {Name: AttrHeight, Type: AttributeTypeInt, Filterable: true},
			AttrCamera:   {Name: AttrCamera, Type: AttributeTypeString, Filterable: true},
			AttrLens:     {Name: AttrLens, Type: AttributeTypeString, Filterable: true},
			AttrISO:      {Name: AttrISO, Type: AttributeTypeInt, Filterable: true},
			AttrAperture: {Name: AttrAperture, Type: AttributeTypeString, Filterable: true},
			AttrGPS:      {Name: AttrGPS, Type: AttributeTypeGPS, Filterable: true},
		},
	},
	{
		Name:        TypeMediaAssetVideo,
		Parent:      TypeMediaAsset,
		Description: "Video asset",
		Attributes: map[string]AttributeDef{
			AttrDuration: {Name: AttrDuration, Type: AttributeTypeInt64, Filterable: true},
			AttrWidth:    {Name: AttrWidth, Type: AttributeTypeInt, Filterable: true},
			AttrHeight:   {Name: AttrHeight, Type: AttributeTypeInt, Filterable: true},
		},
	},

	// Collection types
	{
		Name:        TypeCollectionAlbum,
		Parent:      TypeCollection,
		Description: "Album collection",
		Attributes: map[string]AttributeDef{
			AttrAlbum:      {Name: AttrAlbum, Type: AttributeTypeString, Filterable: true},
			AttrAssetCount: {Name: "asset_count", Type: AttributeTypeInt, Filterable: true},
			AttrCreated:    {Name: AttrCreated, Type: AttributeTypeTime, Filterable: true},
		},
	},
	{
		Name:        TypeCollectionFolder,
		Parent:      TypeCollection,
		Description: "Folder collection",
		Attributes: map[string]AttributeDef{
			AttrPath: {Name: AttrPath, Type: AttributeTypeString, Filterable: true},
		},
	},
}

// RegisterCoreTypes registers all core type definitions in the given registry.
func RegisterCoreTypes(registry *TypeRegistry) {
	for _, typeDef := range TypeHierarchy {
		if err := registry.Register(typeDef); err != nil {
			// Type already registered (shouldn't happen with core types)
			continue
		}
	}
}

// GetCoreTypeNames returns a list of all core type names.
func GetCoreTypeNames() []string {
	names := make([]string, len(TypeHierarchy))
	for i, t := range TypeHierarchy {
		names[i] = t.Name
	}
	return names
}

// IsCoreType checks if a type name is a core type.
func IsCoreType(typeName string) bool {
	for _, t := range TypeHierarchy {
		if t.Name == typeName {
			return true
		}
	}
	return false
}
