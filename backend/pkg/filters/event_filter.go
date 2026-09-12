package filters

import (
	"strings"

	"github.com/jfxdev/gardarr/internal/entities"
)

// MatchesEvent checks if an event matches the filter criteria
// Returns true if the event should be processed, false if it should be filtered out
// An empty filter (all fields empty) matches all events
func MatchesEvent(filter *entities.EventFilter, event *entities.Event) bool {
	if filter == nil {
		// No filter means all events pass
		return true
	}

	// If all filter fields are empty, match all events
	if filter.EventTypeFilter == "" && len(filter.StatusFilter) == 0 &&
		len(filter.CategoryFilter) == 0 && len(filter.NameTerms) == 0 {
		return true
	}

	// Check event type filter (single value, case-insensitive)
	if filter.EventTypeFilter != "" {
		if !strings.EqualFold(filter.EventTypeFilter, event.Type) {
			return false
		}
	}
	// Reports are global events. Torrent-only filters must not suppress them
	// after their event type has been selected.
	if strings.HasPrefix(event.Type, "report.") {
		return true
	}

	// Check status filter (new_value)
	if len(filter.StatusFilter) > 0 {
		if !contains(filter.StatusFilter, event.NewValue) {
			return false
		}
	}

	// Check category filter (from metadata)
	if len(filter.CategoryFilter) > 0 {
		category := extractCategory(event)
		if category == "" || !contains(filter.CategoryFilter, category) {
			return false
		}
	}

	// Check name terms (task name must contain at least one term)
	if len(filter.NameTerms) > 0 {
		taskName := extractTaskName(event)
		if taskName == "" || !containsAnyTerm(taskName, filter.NameTerms) {
			return false
		}
	}

	return true
}

// extractCategory extracts the category from event metadata
func extractCategory(event *entities.Event) string {
	if event.Metadata == nil {
		return ""
	}

	if category, ok := event.Metadata["category"].(string); ok {
		return category
	}

	return ""
}

// extractTaskName extracts the task name from event metadata
func extractTaskName(event *entities.Event) string {
	if event.Metadata == nil {
		return ""
	}

	if name, ok := event.Metadata["name"].(string); ok {
		return name
	}

	return ""
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// containsAnyTerm checks if the text contains any of the terms (case-insensitive)
func containsAnyTerm(text string, terms []string) bool {
	lowerText := strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(lowerText, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
