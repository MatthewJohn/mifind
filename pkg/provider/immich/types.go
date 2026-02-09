package immich

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// FlexibleFloat64 handles unmarshaling from either string or float64.
// Immich API returns duration in various formats:
// - Empty string "" for images (no duration)
// - Time duration string "0:00:00.00000" for videos
// - Direct number (float64)
type FlexibleFloat64 float64

// UnmarshalJSON implements json.Unmarshaler for FlexibleFloat64.
func (f *FlexibleFloat64) UnmarshalJSON(data []byte) error {
	// Handle null
	if len(data) == 0 || string(data) == "null" {
		*f = 0
		return nil
	}
	// Handle empty string
	if string(data) == `""` {
		*f = 0
		return nil
	}
	// Handle string value
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		// Try to parse as float first
		val, err := strconv.ParseFloat(s, 64)
		if err == nil {
			*f = FlexibleFloat64(val)
			return nil
		}
		// Try to parse as duration (format: "0:00:00.00000")
		// Parse it manually since time.ParseDuration doesn't handle HH:MM:SS.mmmm
		duration, err := parseDurationString(s)
		if err != nil {
			return err
		}
		*f = FlexibleFloat64(duration.Seconds())
		return nil
	}
	// Handle direct number
	var val float64
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	*f = FlexibleFloat64(val)
	return nil
}

// parseDurationString parses a duration string in format "HH:MM:SS.mmmm" or "MM:SS.mmmm"
func parseDurationString(s string) (time.Duration, error) {
	// Try standard duration format first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Parse "HH:MM:SS.mmmm" or "0:MM:SS.mmmm" format
	// Split by colons
	var hours, minutes, seconds float64
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		fmt.Sscanf(parts[0], "%f", &hours)
		fmt.Sscanf(parts[1], "%f", &minutes)
		fmt.Sscanf(parts[2], "%f", &seconds)
		return time.Duration(hours*3600+minutes*60+seconds) * time.Second, nil
	}
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%f", &minutes)
		fmt.Sscanf(parts[1], "%f", &seconds)
		return time.Duration(minutes*60+seconds) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid duration format: %s", s)
}

// Value returns the float64 value.
func (f FlexibleFloat64) Value() float64 {
	return float64(f)
}

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
	FileSize         int64          `json:"fileSize,omitempty"`
	Duration         FlexibleFloat64 `json:"duration,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
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
