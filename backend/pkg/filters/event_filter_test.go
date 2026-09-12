package filters

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
)

func TestMatchesEventNilFilter(t *testing.T) {
	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
	}

	if !MatchesEvent(nil, event) {
		t.Error("Expected nil filter to match all events")
	}
}

func TestMatchesEventEmptyFilter(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		StatusFilter:   []string{},
		CategoryFilter: []string{},
		NameTerms:      []string{},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "downloading",
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected empty filter to match all events")
	}
}

func TestMatchesEventStatusFilterMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:         uuid.New(),
		StatusFilter: []string{"completed", "seeding"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event with status 'completed' to match filter")
	}
}

func TestMatchesEventStatusFilterNoMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:         uuid.New(),
		StatusFilter: []string{"completed", "seeding"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "downloading",
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event with status 'downloading' to NOT match filter")
	}
}

func TestMatchesEventCategoryFilterMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		CategoryFilter: []string{"movies", "tv-shows"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "movies",
			"name":     "Test Movie",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event with category 'movies' to match filter")
	}
}

func TestMatchesEventCategoryFilterNoMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		CategoryFilter: []string{"movies", "tv-shows"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "music",
			"name":     "Test Album",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event with category 'music' to NOT match filter")
	}
}

func TestMatchesEventCategoryFilterMissingCategory(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		CategoryFilter: []string{"movies"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"name": "Test",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event without category to NOT match category filter")
	}
}

func TestMatchesEventNameTermsMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:      uuid.New(),
		NameTerms: []string{"ubuntu", "debian"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"name": "Ubuntu 22.04 LTS Desktop",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event with name containing 'ubuntu' to match filter")
	}
}

func TestMatchesEventNameTermsCaseInsensitive(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:      uuid.New(),
		NameTerms: []string{"UBUNTU"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"name": "ubuntu-22.04-desktop",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected case-insensitive name matching")
	}
}

func TestMatchesEventNameTermsNoMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:      uuid.New(),
		NameTerms: []string{"ubuntu", "debian"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"name": "Fedora 38 Workstation",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event with name not containing search terms to NOT match filter")
	}
}

func TestMatchesEventNameTermsMissingName(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:      uuid.New(),
		NameTerms: []string{"ubuntu"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "linux",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event without name to NOT match name filter")
	}
}

func TestMatchesEventCombinedFiltersAllMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		StatusFilter:   []string{"completed"},
		CategoryFilter: []string{"movies"},
		NameTerms:      []string{"2024"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "movies",
			"name":     "Great Movie 2024",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event matching all filter criteria to pass")
	}
}

func TestMatchesEventCombinedFiltersStatusMismatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		StatusFilter:   []string{"completed"},
		CategoryFilter: []string{"movies"},
		NameTerms:      []string{"2024"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "downloading",
		Metadata: map[string]interface{}{
			"category": "movies",
			"name":     "Great Movie 2024",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event with mismatched status to NOT pass filter")
	}
}

func TestMatchesEventCombinedFiltersCategoryMismatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		StatusFilter:   []string{"completed"},
		CategoryFilter: []string{"movies"},
		NameTerms:      []string{"2024"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "tv-shows",
			"name":     "Great Show 2024",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event with mismatched category to NOT pass filter")
	}
}

func TestMatchesEventCombinedFiltersNameMismatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		StatusFilter:   []string{"completed"},
		CategoryFilter: []string{"movies"},
		NameTerms:      []string{"2024"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "movies",
			"name":     "Old Movie 1999",
		},
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event with mismatched name term to NOT pass filter")
	}
}

func TestMatchesEventPartialFilters(t *testing.T) {
	// Test with only status filter
	filter := &entities.EventFilter{
		UUID:         uuid.New(),
		StatusFilter: []string{"completed"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "completed",
		Metadata: map[string]interface{}{
			"category": "anything",
			"name":     "Anything Goes",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected partial filter (status only) to match when status matches")
	}
}

func TestMatchesEventRealWorldScenarioCompletedTorrents(t *testing.T) {
	// Filter for completed torrents only
	filter := &entities.EventFilter{
		UUID:         uuid.New(),
		StatusFilter: []string{"completed", "seeding"},
	}

	completedEvent := &entities.Event{
		UUID:      uuid.New(),
		WorkerID:  uuid.New(),
		Type:      constants.EventTypeTorrentStateChange,
		TaskHash:  "abc123",
		OldValue:  "downloading",
		NewValue:  "completed",
		CreatedAt: time.Now(),
		Metadata: map[string]interface{}{
			"name":     "Ubuntu 22.04 LTS",
			"category": "linux",
			"size":     4294967296,
		},
	}

	if !MatchesEvent(filter, completedEvent) {
		t.Error("Expected completed torrent to match filter")
	}

	downloadingEvent := &entities.Event{
		UUID:      uuid.New(),
		WorkerID:  uuid.New(),
		Type:      constants.EventTypeTorrentStateChange,
		TaskHash:  "def456",
		OldValue:  "paused",
		NewValue:  "downloading",
		CreatedAt: time.Now(),
		Metadata: map[string]interface{}{
			"name":     "Debian 12",
			"category": "linux",
			"size":     3221225472,
		},
	}

	if MatchesEvent(filter, downloadingEvent) {
		t.Error("Expected downloading torrent to NOT match completed-only filter")
	}
}

func TestMatchesEventRealWorldScenarioMovieCategory(t *testing.T) {
	// Filter for movies category only
	filter := &entities.EventFilter{
		UUID:           uuid.New(),
		CategoryFilter: []string{"movies"},
	}

	movieEvent := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "downloading",
		Metadata: map[string]interface{}{
			"name":     "Awesome Movie 2024",
			"category": "movies",
		},
	}

	if !MatchesEvent(filter, movieEvent) {
		t.Error("Expected movie event to match movies category filter")
	}

	tvShowEvent := &entities.Event{
		UUID:     uuid.New(),
		NewValue: "downloading",
		Metadata: map[string]interface{}{
			"name":     "Great TV Show S01E01",
			"category": "tv-shows",
		},
	}

	if MatchesEvent(filter, tvShowEvent) {
		t.Error("Expected TV show event to NOT match movies category filter")
	}
}

func TestMatchesEventEventTypeFilterMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentCompleted,
	}

	event := &entities.Event{
		UUID: uuid.New(),
		Type: constants.EventTypeTorrentCompleted,
	}

	if !MatchesEvent(filter, event) {
		t.Errorf("Expected event with type '%s' to match filter", constants.EventTypeTorrentCompleted)
	}
}

func TestMatchesEventEventTypeFilterNoMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentCompleted,
	}

	event := &entities.Event{
		UUID: uuid.New(),
		Type: constants.EventTypeTorrentAdded,
	}

	if MatchesEvent(filter, event) {
		t.Errorf("Expected event with type '%s' to NOT match '%s' filter", constants.EventTypeTorrentAdded, constants.EventTypeTorrentCompleted)
	}
}

func TestMatchesEventEventTypeFilterCaseInsensitive(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: "TORRENT.COMPLETED",
	}

	event := &entities.Event{
		UUID: uuid.New(),
		Type: constants.EventTypeTorrentCompleted,
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event type filter to be case-insensitive")
	}
}

func TestMatchesEventLeavesSelectedTransferReportsUnaffectedByTorrentFilters(t *testing.T) {
	filter := &entities.EventFilter{
		EventTypeFilter: constants.EventTypeTransferReportDaily,
		StatusFilter:    []string{"UPLOADING"},
		CategoryFilter:  []string{"movies"},
		NameTerms:       []string{"linux"},
	}
	event := &entities.Event{Type: constants.EventTypeTransferReportDaily}
	if !MatchesEvent(filter, event) {
		t.Fatal("selected transfer report should bypass torrent-only filters")
	}
	filter.EventTypeFilter = constants.EventTypeTransferReportWeekly
	if MatchesEvent(filter, event) {
		t.Fatal("different transfer report type should not match")
	}
}

func TestMatchesEventEventTypeAndStatusFilterBothMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentStateChange,
		StatusFilter:    []string{"UPLOADING", "DOWNLOADING"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		Type:     constants.EventTypeTorrentStateChange,
		NewValue: "UPLOADING",
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event to match both event type and status filters")
	}
}

func TestMatchesEventEventTypeAndStatusFilterEventTypeNoMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentCompleted,
		StatusFilter:    []string{"UPLOADING"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		Type:     constants.EventTypeTorrentStateChange,
		NewValue: "UPLOADING",
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event to NOT match when event type doesn't match")
	}
}

func TestMatchesEventEventTypeAndStatusFilterStatusNoMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentStateChange,
		StatusFilter:    []string{"UPLOADING"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		Type:     constants.EventTypeTorrentStateChange,
		NewValue: "DOWNLOADING",
	}

	if MatchesEvent(filter, event) {
		t.Error("Expected event to NOT match when status doesn't match")
	}
}

func TestMatchesEventEventTypeAndCategoryFilterBothMatch(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentAdded,
		CategoryFilter:  []string{"movies"},
	}

	event := &entities.Event{
		UUID: uuid.New(),
		Type: constants.EventTypeTorrentAdded,
		Metadata: map[string]interface{}{
			"category": "movies",
			"name":     "Test Movie",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Errorf("Expected event to match both event type '%s' and category filters", constants.EventTypeTorrentAdded)
	}
}

func TestMatchesEventAllFiltersApplied(t *testing.T) {
	filter := &entities.EventFilter{
		UUID:            uuid.New(),
		EventTypeFilter: constants.EventTypeTorrentStateChange,
		StatusFilter:    []string{"DOWNLOADING"},
		CategoryFilter:  []string{"movies"},
		NameTerms:       []string{"ubuntu"},
	}

	event := &entities.Event{
		UUID:     uuid.New(),
		Type:     constants.EventTypeTorrentStateChange,
		NewValue: "DOWNLOADING",
		Metadata: map[string]interface{}{
			"category": "movies",
			"name":     "Ubuntu Movie Edition",
		},
	}

	if !MatchesEvent(filter, event) {
		t.Error("Expected event to match all filters")
	}

	// Test with one filter not matching
	eventWrongCategory := &entities.Event{
		UUID:     uuid.New(),
		Type:     constants.EventTypeTorrentStateChange,
		NewValue: "DOWNLOADING",
		Metadata: map[string]interface{}{
			"category": "tv-shows",
			"name":     "Ubuntu Movie Edition",
		},
	}

	if MatchesEvent(filter, eventWrongCategory) {
		t.Error("Expected event to NOT match when category filter doesn't match")
	}
}
