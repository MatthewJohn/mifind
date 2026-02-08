package immich

import (
	"time"

	"github.com/yourname/mifind/internal/types"
)

// API types for the Immich API responses.
// See https://github.com/immich-app/immich/tree/main/server/api-openapi

// Asset represents a media asset (photo or video) in Immich.
type Asset struct {
	ID               string    `json:"id"`
	DeviceAssetID    string    `json:"deviceAssetId"`
	Type             string    `json:"type"` // "IMAGE" or "VIDEO"
	OriginalPath     string    `json:"originalPath"`
	OriginalFileName string    `json:"originalFileName"`
	Width            int       `json:"width,omitempty"`
	Height           int       `json:"height,omitempty"`
	ExifInfo         *ExifInfo `json:"exifInfo,omitempty"`
	Thumbhash        string    `json:"thumbhash,omitempty"`
	FileSize         int64     `json:"fileSize,omitempty"`
	Duration         float64   `json:"duration,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	LocalDateTime    time.Time `json:"localDateTime"`
	IsFavorite       bool      `json:"isFavorite"`
	IsArchived       bool      `json:"isArchived"`
	Description      string    `json:"description,omitempty"`
	Location         string    `json:"location,omitempty"`
}

// ExifInfo contains EXIF metadata for photos.
type ExifInfo struct {
	Make             string   `json:"make,omitempty"`
	Model            string   `json:"model,omitempty"`
	ExifImageWidth   int      `json:"exifImageWidth,omitempty"`
	ExifImageHeight  int      `json:"exifImageHeight,omitempty"`
	FNumber          float32  `json:"fNumber,omitempty"`
	ExposureTime     string   `json:"exposureTime,omitempty"`
	ISOSpeed         int      `json:"iso,omitempty"`
	FocalLength      float32  `json:"focalLength,omitempty"`
	LensModel        string   `json:"lensModel,omitempty"`
	DateTimeOriginal string   `json:"dateTimeOriginal,omitempty"`
	GPS              *GPSInfo `json:"exifGPSLatitude,omitempty"`
}

// GPSInfo contains GPS coordinates.
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Album represents a collection of assets in Immich.
type Album struct {
	ID          string    `json:"id"`
	AlbumName   string    `json:"albumName"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	AssetCount  int       `json:"assetCount,omitempty"`
	StartDate   time.Time `json:"startDate,omitempty"`
	EndDate     time.Time `json:"endDate,omitempty"`
}

// Person represents a detected person in Immich.
type Person struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	BirthDate     string    `json:"birthDate,omitempty"`
	ThumbnailPath string    `json:"thumbnailPath,omitempty"`
	IsHidden      bool      `json:"isHidden"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// SearchResponse represents search results from Immich.
type SearchResponse struct {
	Assets *AssetSearchResponse  `json:"assets,omitempty"`
	Albums *AlbumSearchResponse  `json:"albums,omitempty"`
	People *PeopleSearchResponse `json:"people,omitempty"`
}

// AssetSearchResponse contains asset search results.
type AssetSearchResponse struct {
	Total   int     `json:"total"`
	Count   int     `json:"count"`
	Items   []Asset `json:"items"`
	Expires string  `json:"expires,omitempty"`
}

// AlbumSearchResponse contains album search results.
type AlbumSearchResponse struct {
	Total int     `json:"total"`
	Count int     `json:"count"`
	Items []Album `json:"items"`
}

// PeopleSearchResponse contains people search results.
type PeopleSearchResponse struct {
	Total int      `json:"total"`
	Count int      `json:"count"`
	Items []Person `json:"items"`
}

// AssetBulkUploadCheckResponse represents the response for bulk upload check.
type AssetBulkUploadCheckResponse struct {
	Results []AssetBulkUploadCheckResult `json:"results"`
}

// AssetBulkUploadCheckResult represents a single check result.
type AssetBulkUploadCheckResult struct {
	ID     string `json:"id"`
	Action string `json:"action"` // "accept", "reject"
	Reason string `json:"reason,omitempty"`
	Asset  *Asset `json:"asset,omitempty"`
}

// Entity type constants.
const (
	EntityTypePhoto = "IMAGE"
	EntityTypeVideo = "VIDEO"
)

// FileTypeToMifindType converts an Immich asset type to a mifind core type.
func FileTypeToMifindType(assetType string) string {
	switch assetType {
	case EntityTypePhoto:
		return types.TypeMediaAssetPhoto
	case EntityTypeVideo:
		return types.TypeMediaAssetVideo
	default:
		return types.TypeMediaAsset
	}
}
