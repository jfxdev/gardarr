package schemas

// CategoryCreateRequest represents the request body for creating a category
type CategoryCreateRequest struct {
	Name               string   `json:"name" binding:"required,min=1,max=100"`
	DefaultTags        []string `json:"default_tags"`
	DefaultDirectories []string `json:"default_directories" binding:"omitempty,max=50,dive,min=1,max=255"`
	MetadataSource     string   `json:"metadata_source" binding:"omitempty,oneof=none tgdb tmdb"`
	ReleaseType        string   `json:"release_type" binding:"omitempty,oneof=none movie series os game book music software audiobook comic course dataset rom podcast anime"`
	Color              string   `json:"color" binding:"omitempty,max=50"`
	Icon               string   `json:"icon" binding:"omitempty,max=100"`
}

// CategoryUpdateRequest represents the request body for updating a category
// Note: Name and ID are immutable and cannot be updated
type CategoryUpdateRequest struct {
	DefaultTags        []string `json:"default_tags"`
	DefaultDirectories []string `json:"default_directories" binding:"omitempty,max=50,dive,min=1,max=255"`
	MetadataSource     string   `json:"metadata_source" binding:"omitempty,oneof=none tgdb tmdb"`
	ReleaseType        string   `json:"release_type" binding:"omitempty,oneof=none movie series os game book music software audiobook comic course dataset rom podcast anime"`
	Color              string   `json:"color" binding:"omitempty,max=50"`
	Icon               string   `json:"icon" binding:"omitempty,max=100"`
}
