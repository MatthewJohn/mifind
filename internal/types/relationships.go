package types

// Standard relationship types that should be used across providers
// for common connections between entities.
const (
	// General containment relationships
	RelParent  = "parent"  // Parent entity (e.g., folder containing file)
	RelChild   = "child"   // Child entity (inverse of parent)
	RelSibling = "sibling" // Sibling entity (same parent)

	// Collection membership
	RelCollection = "collection" // Member of a collection
	RelAlbum      = "album"      // Member of an album
	RelFolder     = "folder"     // Contained in folder
	RelPlaylist   = "playlist"   // Member of a playlist

	// Media/creative relationships
	RelArtist     = "artist"      // Created by artist
	RelAlbumOwner = "album_owner" // Album owned by artist
	RelTrack      = "track"       // Track on album
	RelSeries     = "series"      // Part of a series
	RelSeason     = "season"      // Part of a season (TV)
	RelEpisode    = "episode"     // Episode in a season
	RelMovie      = "movie"       // Movie in a collection

	// File/media representation relationships
	RelOriginalFile = "original_file" // Original file
	RelThumbnail    = "thumbnail"     // Thumbnail representation
	RelPreview      = "preview"       // Preview representation
	RelTranscode    = "transcode"     // Transcoded version

	// Version control relationships
	RelRepository   = "repository"    // Belongs to repository
	RelBranch       = "branch"        // On branch
	RelCommit       = "commit"        // Related commit
	RelParentCommit = "parent_commit" // Parent commit
	RelDiff         = "diff"          // Diff between versions

	// Social/people relationships
	RelOwner       = "owner"       // Owned by
	RelCreator     = "creator"     // Created by
	RelContributor = "contributor" // Contributed by
	RelAssignee    = "assignee"    // Assigned to
	RelMentioned   = "mentioned"   // Mentioned in
	RelFollower    = "follower"    // Followed by
	RelFollowing   = "following"   // Following

	// Tag/categorization relationships
	RelTag      = "tag"      // Has tag
	RelCategory = "category" // In category
	RelTopic    = "topic"    // About topic

	// Location relationships
	RelLocation    = "location"     // At location
	RelPlace       = "place"        // At place
	RelVenue       = "venue"        // At venue
	RelContainedIn = "contained_in" // Contained in larger place

	// Temporal relationships
	RelBefore   = "before"   // Before another entity
	RelAfter    = "after"    // After another entity
	RelOverlaps = "overlaps" // Overlaps in time

	// Reference/link relationships
	RelReferences   = "references"    // References another entity
	RelReferencedBy = "referenced_by" // Referenced by another entity
	RelRelatedTo    = "related_to"    // Generally related
	RelSimilarTo    = "similar_to"    // Similar to another entity
	RelDuplicateOf  = "duplicate_of"  // Duplicate of another entity

	// Dependency relationships
	RelDependsOn   = "depends_on"   // Depends on another entity
	RelRequiredBy  = "required_by"  // Required by another entity
	RelDerivedFrom = "derived_from" // Derived from another entity
	RelReplaces    = "replaces"     // Replaces another entity
	RelReplacedBy  = "replaced_by"  // Replaced by another entity

	// Face/person detection relationships (Immich, etc.)
	RelPerson  = "person"   // Person detected in media
	RelFace    = "face"     // Face region in media
	RelInMedia = "in_media" // Person appears in media
)

// RelationshipDirection indicates the direction of a relationship.
type RelationshipDirection string

const (
	// DirectionOutgoing means the relationship points from this entity to another
	DirectionOutgoing RelationshipDirection = "outgoing"
	// DirectionIncoming means the relationship points from another entity to this one
	DirectionIncoming RelationshipDirection = "incoming"
	// DirectionBoth means both directions should be considered
	DirectionBoth RelationshipDirection = "both"
)

// RelationshipQuery defines a query for relationships.
type RelationshipQuery struct {
	// Type is the relationship type to query (empty for all types)
	Type string

	// Direction specifies which direction to query
	Direction RelationshipDirection

	// Limit limits the number of results
	Limit int

	// Offset skips the first N results
	Offset int
}

// RelationshipResult contains relationship query results.
type RelationshipResult struct {
	// Type is the relationship type
	Type string

	// Entity is the related entity
	Entity Entity

	// Direction indicates the relationship direction
	Direction RelationshipDirection
}

// Standard filter definitions for relationship types.
// These can be used to build relationship filter UIs.
var (
	// FilterByAlbum filters by album relationship.
	FilterByAlbum = FilterDefinition{
		Name:        RelAlbum,
		Type:        FilterTypeSelect,
		Label:       "Album",
		Description: "Filter by album",
	}

	// FilterByArtist filters by artist relationship.
	FilterByArtist = FilterDefinition{
		Name:        RelArtist,
		Type:        FilterTypeSelect,
		Label:       "Artist",
		Description: "Filter by artist",
	}

	// FilterByFolder filters by folder relationship.
	FilterByFolder = FilterDefinition{
		Name:        RelFolder,
		Type:        FilterTypeSelect,
		Label:       "Folder",
		Description: "Filter by folder",
	}

	// FilterByLocation filters by location relationship.
	FilterByLocation = FilterDefinition{
		Name:        RelLocation,
		Type:        FilterTypeSelect,
		Label:       "Location",
		Description: "Filter by location",
	}

	// FilterByOwner filters by owner relationship.
	FilterByOwner = FilterDefinition{
		Name:        RelOwner,
		Type:        FilterTypeSelect,
		Label:       "Owner",
		Description: "Filter by owner",
	}

	// FilterByTag filters by tag relationship.
	FilterByTag = FilterDefinition{
		Name:        RelTag,
		Type:        FilterTypeMulti,
		Label:       "Tags",
		Description: "Filter by tags",
	}

	// FilterByPerson filters by person relationship.
	FilterByPerson = FilterDefinition{
		Name:        RelPerson,
		Type:        FilterTypeMulti,
		Label:       "People",
		Description: "Filter by detected people",
	}
)

// IsBidirectional returns true if a relationship type is typically bidirectional.
func IsBidirectional(relType string) bool {
	switch relType {
	case RelSibling, RelRelatedTo, RelSimilarTo, RelOverlaps:
		return true
	default:
		return false
	}
}

// GetInverseRelationship returns the inverse relationship type.
// Returns empty string if no standard inverse exists.
func GetInverseRelationship(relType string) string {
	inverses := map[string]string{
		RelParent:       RelChild,
		RelChild:        RelParent,
		RelReferences:   RelReferencedBy,
		RelReferencedBy: RelReferences,
		RelDependsOn:    RelRequiredBy,
		RelRequiredBy:   RelDependsOn,
		RelDerivedFrom:  RelReplaces,
		RelReplaces:     RelReplacedBy,
		RelReplacedBy:   RelReplaces,
		RelBefore:       RelAfter,
		RelAfter:        RelBefore,
		RelInMedia:      RelPerson,
		RelPerson:       RelInMedia,
	}
	return inverses[relType]
}
