package task

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/go-qbt"
)

func TestRepositoryAddBase32Magnet(t *testing.T) {
	const hash = "5a0cdacbd5571efcdd5e279fcb0e913b6c36cb36"
	const magnet = "magnet:?xt=urn:btih:lignvs6vk4ppzxk6e6p4wdurhnwdnszw&dn=Example"
	var addedMagnet, taggedHash string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/torrents/add":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			addedMagnet = r.Form.Get("urls")
		case "/api/v2/torrents/info":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"hash": hash, "name": "Example", "magnet_uri": "magnet:?xt=urn:btih:" + hash,
			}})
		case "/api/v2/torrents/addTags":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			taggedHash = r.Form.Get("hashes")
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	task, err := New(client).Add(schemas.TaskCreateSchema{MagnetURI: magnet, Tags: []string{"movie"}})
	if err != nil {
		t.Fatal(err)
	}
	if task.Hash != hash || addedMagnet != magnet || taggedHash != hash {
		t.Fatalf("unexpected task/add/tags: %#v, %q, %q", task, addedMagnet, taggedHash)
	}
}

func TestParseMagnetLinkBase32(t *testing.T) {
	const hash = "5a0cdacbd5571efcdd5e279fcb0e913b6c36cb36"
	for _, encoded := range []string{"lignvs6vk4ppzxk6e6p4wdurhnwdnszw", "LIGNVS6VK4PPZXK6E6P4WDURHNWDNSZW", hash} {
		t.Run(encoded, func(t *testing.T) {
			uri, err := ParseMagnetLink("magnet:?xt=urn:btih:" + encoded + "&dn=Gardarr Test Fixture.2026.1080p.WEB-DL.x264&tr=udp://tracker.example.invalid:1337/announce")
			if err != nil {
				t.Fatal(err)
			}
			if uri.Hash != hash || uri.DisplayName != "Gardarr Test Fixture.2026.1080p.WEB-DL.x264" || !reflect.DeepEqual(uri.Trackers, []string{"udp://tracker.example.invalid:1337/announce"}) {
				t.Fatalf("unexpected parsed magnet: %#v", uri)
			}
		})
	}
	if _, err := ParseMagnetLink("magnet:?xt=urn:btih:00000000000000000000000000000000"); err == nil {
		t.Fatal("expected invalid Base32 hash to be rejected")
	}
}

func TestDiffTaskTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		current      string
		desired      []string
		wantToAdd    []string
		wantToRemove []string
	}{
		{
			name:         "adds new tags when current is empty",
			current:      "",
			desired:      []string{"a", "b"},
			wantToAdd:    []string{"a", "b"},
			wantToRemove: []string{},
		},
		{
			name:         "removes tags not present anymore",
			current:      "a,b,c",
			desired:      []string{"b"},
			wantToAdd:    []string{},
			wantToRemove: []string{"a", "c"},
		},
		{
			name:         "mixes additions and removals",
			current:      "a,b",
			desired:      []string{"b", "c"},
			wantToAdd:    []string{"c"},
			wantToRemove: []string{"a"},
		},
		{
			name:         "clears all tags when desired is empty",
			current:      "a,b",
			desired:      []string{},
			wantToAdd:    []string{},
			wantToRemove: []string{"a", "b"},
		},
		{
			name:         "normalizes duplicates and whitespace",
			current:      " a, b ,b",
			desired:      []string{" b ", "c", "", "c"},
			wantToAdd:    []string{"c"},
			wantToRemove: []string{"a"},
		},
		{
			// A full desired list naturally enforces scope exclusivity: the
			// old scoped value isn't repeated in desired, so the diff drops
			// it like any other tag the caller no longer wants.
			name:         "replaces a scoped value not repeated in the desired list",
			current:      "quality::1080p,movies",
			desired:      []string{"movies", "quality::4k"},
			wantToAdd:    []string{"quality::4k"},
			wantToRemove: []string{"quality::1080p"},
		},
		{
			// If the caller's own desired list holds two values for the
			// same scope, normalizeDesiredTags keeps only the last one
			// before the diff even runs - here that's the value already
			// on the torrent, so nothing changes.
			name:         "desired list with its own scope conflict keeps only the last value",
			current:      "quality::1080p",
			desired:      []string{"quality::4k", "quality::1080p"},
			wantToAdd:    []string{},
			wantToRemove: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotToAdd, gotToRemove := diffTaskTags(tt.current, tt.desired)

			if !reflect.DeepEqual(gotToAdd, tt.wantToAdd) {
				t.Fatalf("tagsToAdd mismatch: got %v want %v", gotToAdd, tt.wantToAdd)
			}

			if !reflect.DeepEqual(gotToRemove, tt.wantToRemove) {
				t.Fatalf("tagsToRemove mismatch: got %v want %v", gotToRemove, tt.wantToRemove)
			}
		})
	}
}

func TestResolveScopeConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "no scoped tags passes through unchanged",
			tags: []string{"movies", "genre:action"},
			want: []string{"movies", "genre:action"},
		},
		{
			name: "single value per scope is kept",
			tags: []string{"quality::4k", "movies"},
			want: []string{"quality::4k", "movies"},
		},
		{
			name: "later value for the same scope wins",
			tags: []string{"quality::1080p", "movies", "quality::4k"},
			want: []string{"movies", "quality::4k"},
		},
		{
			name: "different scopes don't interfere with each other",
			tags: []string{"quality::1080p", "genre::action", "quality::4k", "genre::comedy"},
			want: []string{"quality::4k", "genre::comedy"},
		},
		{
			name: "grouped tags (single colon) are never exclusive",
			tags: []string{"genre:action", "genre:thriller"},
			want: []string{"genre:action", "genre:thriller"},
		},
		{
			name: "empty input",
			tags: []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveScopeConflicts(tt.tags)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeDesiredTagsEnforcesScopeConflicts(t *testing.T) {
	t.Parallel()

	// normalizeDesiredTags must apply scope-conflict resolution after its
	// own trim/dedup pass, since that's the only path Add/AddFile/SetTags
	// run a brand-new torrent's tag list through.
	got := normalizeDesiredTags([]string{" quality::1080p ", "quality::1080p", "quality::4k"})
	want := []string{"quality::4k"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestConflictingScopedTagsByHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		currentByHash map[string]string
		incoming      []string
		want          map[string][]string
	}{
		{
			name:          "no incoming scoped tags means nothing to check",
			currentByHash: map[string]string{"h1": "quality::1080p"},
			incoming:      []string{"movies", "genre:action"},
			want:          nil,
		},
		{
			name:          "current value under the same scope conflicts",
			currentByHash: map[string]string{"h1": "quality::1080p,movies"},
			incoming:      []string{"quality::4k"},
			want:          map[string][]string{"h1": {"quality::1080p"}},
		},
		{
			name:          "current value equal to incoming is not a conflict",
			currentByHash: map[string]string{"h1": "quality::4k,movies"},
			incoming:      []string{"quality::4k"},
			want:          map[string][]string{},
		},
		{
			name:          "unrelated scopes and plain tags are left alone",
			currentByHash: map[string]string{"h1": "genre::action,movies"},
			incoming:      []string{"quality::4k"},
			want:          map[string][]string{},
		},
		{
			name: "only the torrents that actually conflict are reported",
			currentByHash: map[string]string{
				"h1": "quality::1080p",
				"h2": "quality::4k",
			},
			incoming: []string{"quality::4k"},
			want:     map[string][]string{"h1": {"quality::1080p"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := conflictingScopedTagsByHash(tt.currentByHash, tt.incoming)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %v want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			for hash, want := range tt.want {
				if !reflect.DeepEqual(got[hash], want) {
					t.Fatalf("hash %s: got %v want %v", hash, got[hash], want)
				}
			}
		})
	}
}

func TestNormalizeTaskCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantCategory    string
		wantShouldClear bool
	}{
		{
			name:            "keeps non-empty category",
			input:           "movies",
			wantCategory:    "movies",
			wantShouldClear: false,
		},
		{
			name:            "trims surrounding whitespace",
			input:           "  series  ",
			wantCategory:    "series",
			wantShouldClear: false,
		},
		{
			name:            "marks empty category for removal",
			input:           "   ",
			wantCategory:    "",
			wantShouldClear: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotCategory, gotShouldClear := normalizeTaskCategory(tt.input)
			if gotCategory != tt.wantCategory {
				t.Fatalf("unexpected category: got %q want %q", gotCategory, tt.wantCategory)
			}
			if gotShouldClear != tt.wantShouldClear {
				t.Fatalf("unexpected shouldClear: got %v want %v", gotShouldClear, tt.wantShouldClear)
			}
		})
	}
}

func TestMatchCategoryNameIgnoresCase(t *testing.T) {
	categories := map[string]qbt.Category{
		"Movies": {},
		"series": {Name: "series"},
	}

	if got := matchCategoryName(categories, "movies"); got != "Movies" {
		t.Fatalf("expected qBittorrent spelling %q, got %q", "Movies", got)
	}
}

func TestMatchCategoryNameKeepsUnknownCategory(t *testing.T) {
	categories := map[string]qbt.Category{
		"Movies": {Name: "Movies"},
	}

	if got := matchCategoryName(categories, "documentaries"); got != "documentaries" {
		t.Fatalf("expected unknown category to remain unchanged, got %q", got)
	}
}
