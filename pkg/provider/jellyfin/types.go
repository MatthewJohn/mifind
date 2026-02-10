package jellyfin

import "time"

// Item represents a media item in Jellyfin.
type Item struct {
	ID              string     `json:"Id"`
	Name            string     `json:"Name"`
	OriginalTitle   string     `json:"OriginalTitle"`
	Type            string     `json:"Type"` // Movie, Series, Season, Episode
	SeriesName      string     `json:"SeriesName"`
	SeasonName      string     `json:"SeasonName"`
	PremiereDate    *time.Time `json:"PremiereDate"`
	ProductionYear  int        `json:"ProductionYear"`
	Genres          []string   `json:"Genres"`
	Studios         []Studio   `json:"Studios"`
	Overview        string     `json:"Overview"`
	CommunityRating float64    `json:"CommunityRating"`
	CriticRating    float64    `json:"CriticRating"`
	RunTimeTicks    int64      `json:"RunTimeTicks"`
	OfficialRating  string     `json:"OfficialRating"`
	ProductionLocations []string `json:"ProductionLocations"`
	Path            string     `json:"Path"`
	IndexNumber     int        `json:"IndexNumber"`     // Episode or season number
	ParentIndexNumber int      `json:"ParentIndexNumber"` // Season number for episodes
}

// Studio represents a studio in Jellyfin.
type Studio struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

// ItemsResponse represents the response from the Items endpoint.
type ItemsResponse struct {
	Items      []Item `json:"Items"`
	TotalCount int    `json:"TotalRecordCount"`
}

// UserInfo represents information about the current user.
type UserInfo struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// UserResponse represents the response from the Users/Me endpoint.
type UserResponse struct {
	UserInfo
}

// GenreItemsResponse represents the response from the Genres endpoint.
type GenreItemsResponse struct {
	Items []GenreItem `json:"Items"`
}

// GenreItem represents a genre in Jellyfin.
type GenreItem struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

// StudioItemsResponse represents the response from the Studios endpoint.
type StudioItemsResponse struct {
	Items []StudioItem `json:"Items"`
}

// StudioItem represents a studio item in the list.
type StudioItem struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}
