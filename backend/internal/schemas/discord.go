package schemas

type DiscordCreateRequest struct {
	Name           string   `json:"name" binding:"required,max=100"`
	WebhookURL     string   `json:"webhook_url" binding:"required"`
	Enabled        *bool    `json:"enabled"`
	AllEvents      *bool    `json:"all_events"`
	EventTypes     []string `json:"event_types"`
	StatusFilter   []string `json:"status_filter"`
	CategoryFilter []string `json:"category_filter"`
	NameTerms      []string `json:"name_terms"`
}

type DiscordUpdateRequest struct {
	Name           *string   `json:"name"`
	WebhookURL     *string   `json:"webhook_url"`
	Enabled        *bool     `json:"enabled"`
	AllEvents      *bool     `json:"all_events"`
	EventTypes     *[]string `json:"event_types"`
	StatusFilter   *[]string `json:"status_filter"`
	CategoryFilter *[]string `json:"category_filter"`
	NameTerms      *[]string `json:"name_terms"`
}
