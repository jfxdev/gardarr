package task

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/internal/tagrules"
	"github.com/jfxdev/go-qbt"

	"github.com/jfxdev/gardarr/pkg/errors"
	"github.com/jfxdev/gardarr/pkg/torrentfile"
)

type Repository struct {
	client *qbt.Client
}

func New(client *qbt.Client) *Repository {
	return &Repository{
		client: client,
	}
}

func (s *Repository) List() ([]*entities.Task, error) {
	items, err := s.client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]*entities.Task, len(items))
	for i, item := range items {
		result[i] = toTask(item)
	}

	return result, nil
}

func (s *Repository) Get(hash string) (*entities.Task, error) {
	items, err := s.client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.Hash == hash {
			return toTask(item), nil
		}
	}

	return nil, errors.ErrTaskNotFound
}

func (s *Repository) Add(schema schemas.TaskCreateSchema) (*entities.Task, error) {
	if err := s.client.AddTorrentLink(qbt.TorrentConfig{
		MagnetURI: schema.MagnetURI,
		Category:  schema.Category,
		Directory: schema.Directory,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to add task")
	}

	list, err := s.client.ListTorrents(qbt.ListOptions{
		Category: schema.Category,
	})
	if err != nil {
		return nil, err
	}

	uri, err := ParseMagnetLink(schema.MagnetURI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse magnet link")
	}

	var task *qbt.TorrentResponse
	for _, item := range list {
		if strings.EqualFold(item.Hash, uri.Hash) || strings.EqualFold(item.MagnetLink.Hash, uri.Hash) {
			task = item
			if tags := normalizeDesiredTags(schema.Tags); len(tags) > 0 {
				if err := s.client.AddTorrentTags(task.Hash, tags); err != nil {
					return nil, err
				}
			}
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("failed to retrieve created task")
	}

	return toTask(task), nil
}

// AddFile adds a torrent from an uploaded .torrent file, mirroring Add's
// magnet flow: hashes (the file's info-hashes, already parsed by the caller
// for the pre-add dedupe check) are used to locate the created task for tag
// application and the response.
func (s *Repository) AddFile(fileName string, fileData []byte, hashes []string, schema schemas.TaskCreateFromFileSchema) (*entities.Task, error) {
	if err := s.client.AddTorrentFile(qbt.TorrentFileConfig{
		FileName:  fileName,
		FileData:  fileData,
		Category:  schema.Category,
		Directory: schema.Directory,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to add task from file")
	}

	list, err := s.client.ListTorrents(qbt.ListOptions{
		Category: schema.Category,
	})
	if err != nil {
		return nil, err
	}

	for _, item := range list {
		if torrentfile.MatchesInfoHash(hashes, item.InfoHashV1, item.InfoHashV2) {
			if tags := normalizeDesiredTags(schema.Tags); len(tags) > 0 {
				if err := s.client.AddTorrentTags(item.Hash, tags); err != nil {
					return nil, err
				}
			}
			return toTask(item), nil
		}
	}

	return nil, fmt.Errorf("failed to retrieve created task")
}

func (s *Repository) Stop(hash string) error {
	if err := s.client.StopTorrents(hash); err != nil {
		return errors.Wrap(err, "failed to stop torrent")
	}

	return nil
}

func (s *Repository) Delete(id string, deleteFiles bool) error {
	if err := s.client.DeleteTorrents(id, deleteFiles); err != nil {
		return errors.Wrap(err, "failed to delete torrent")
	}

	return nil
}

func (s *Repository) Start(hash string) error {
	if err := s.client.StartTorrents(hash); err != nil {
		return errors.Wrap(err, "failed to start torrent")
	}

	return nil
}

func (s *Repository) ForceResume(hash string) error {
	if err := s.client.ForceStart(hash); err != nil {
		return errors.Wrap(err, "failed to force resume torrent")
	}

	return nil
}

func (s *Repository) SetTags(hash string, tags []string) error {
	items, err := s.client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.Hash == hash {
			tagsToAdd, tagsToRemove := diffTaskTags(item.Tags, tags)

			if len(tagsToRemove) > 0 {
				if err := s.client.DeleteTorrentTags(hash, tagsToRemove); err != nil {
					return errors.Wrap(err, "failed to remove torrent tags")
				}
			}

			if len(tagsToAdd) > 0 {
				if err := s.client.AddTorrentTags(hash, tagsToAdd); err != nil {
					return errors.Wrap(err, "failed to add torrent tags")
				}
			}

			return nil
		}
	}

	return errors.ErrTaskNotFound
}

func diffTaskTags(current string, desired []string) ([]string, []string) {
	currentList := splitTaskTags(current)
	desiredList := normalizeDesiredTags(desired)

	currentSet := make(map[string]struct{}, len(currentList))
	for _, tag := range currentList {
		currentSet[tag] = struct{}{}
	}

	desiredSet := make(map[string]struct{}, len(desiredList))
	for _, tag := range desiredList {
		desiredSet[tag] = struct{}{}
	}

	tagsToAdd := make([]string, 0)
	for _, tag := range desiredList {
		if _, exists := currentSet[tag]; !exists {
			tagsToAdd = append(tagsToAdd, tag)
		}
	}

	tagsToRemove := make([]string, 0)
	for _, tag := range currentList {
		if _, exists := desiredSet[tag]; !exists {
			tagsToRemove = append(tagsToRemove, tag)
		}
	}

	return tagsToAdd, tagsToRemove
}

func splitTaskTags(tags string) []string {
	if strings.TrimSpace(tags) == "" {
		return []string{}
	}

	result := make([]string, 0)
	seen := make(map[string]struct{})

	for _, tag := range strings.Split(tags, ",") {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

// normalizeDesiredTags trims/dedupes a tag list and enforces scoped-tag
// exclusivity within it (see internal/tagrules): a "key::value" tag is
// exclusive within "key", so if the list holds more than one value for the
// same key, only the last one (in input order) survives. This covers a
// closed desired set on its own - a brand new torrent's tags, or the full
// list SetTags diffs against current - since any conflicting older value
// not repeated in the desired set is naturally dropped by that diff.
func normalizeDesiredTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))

	for _, tag := range tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return resolveScopeConflicts(result)
}

// resolveScopeConflicts keeps only the last tag (in input order) for each
// scope key. tags must already be deduplicated - every element is a
// distinct string, so at most one of them equals lastForScope[scope].
func resolveScopeConflicts(tags []string) []string {
	lastForScope := make(map[string]string)
	for _, tag := range tags {
		if scope, ok := tagrules.ScopeOf(tag); ok {
			lastForScope[scope] = tag
		}
	}

	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		scope, ok := tagrules.ScopeOf(tag)
		if !ok || lastForScope[scope] == tag {
			result = append(result, tag)
		}
	}

	return result
}

// AddTags adds tags to one or more torrents (hash may be "h1|h2|...").
// Unlike SetTags it does not diff the full desired state, but it still
// enforces scoped-tag exclusivity: adding a "key::value" tag to a torrent
// that already carries a different value under the same "key" removes
// that old value first (removeConflictingScopedTags), so a torrent can't
// end up holding two values for the same exclusive scope.
func (s *Repository) AddTags(hash string, tags []string) error {
	normalized := normalizeDesiredTags(tags)
	if len(normalized) == 0 {
		return nil
	}

	if err := s.client.AddTorrentTags(hash, normalized); err != nil {
		return errors.Wrap(err, "failed to add torrent tags")
	}

	if err := s.removeConflictingScopedTags(hash, normalized); err != nil {
		return errors.Wrap(err, "failed to remove conflicting scoped tags")
	}

	return nil
}

// removeConflictingScopedTags strips, per torrent in the "h1|h2|..." batch,
// any current tag whose scope collides with one of the incoming tags but
// whose value differs. It's a no-op (skipping the ListTorrents call
// entirely) unless at least one incoming tag is scoped.
func (s *Repository) removeConflictingScopedTags(hash string, incoming []string) error {
	hasScoped := false
	for _, tag := range incoming {
		if _, ok := tagrules.ScopeOf(tag); ok {
			hasScoped = true
			break
		}
	}
	if !hasScoped {
		return nil
	}

	targeted := make(map[string]struct{})
	for _, h := range strings.Split(hash, "|") {
		if h = strings.TrimSpace(h); h != "" {
			targeted[h] = struct{}{}
		}
	}

	items, err := s.client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		return err
	}

	currentByHash := make(map[string]string, len(targeted))
	for _, item := range items {
		if _, ok := targeted[item.Hash]; ok {
			currentByHash[item.Hash] = item.Tags
		}
	}

	removals := conflictingScopedTagsByHash(currentByHash, incoming)

	// Group hashes that need the exact same tags removed so they can share
	// a single DeleteTorrentTags call instead of one per torrent.
	hashesByTagSet := make(map[string][]string)
	for h, toRemove := range removals {
		slices.Sort(toRemove)
		key := strings.Join(toRemove, ",")
		hashesByTagSet[key] = append(hashesByTagSet[key], h)
	}

	for tagSet, hashes := range hashesByTagSet {
		if err := s.client.DeleteTorrentTags(strings.Join(hashes, "|"), strings.Split(tagSet, ",")); err != nil {
			return err
		}
	}

	return nil
}

// conflictingScopedTagsByHash computes, for each hash in currentByHash,
// which of its current tags need removing before the incoming tags are
// added: a current tag conflicts if it shares a scope with an incoming
// tag but has a different value.
func conflictingScopedTagsByHash(currentByHash map[string]string, incoming []string) map[string][]string {
	incomingByScope := make(map[string]string, len(incoming))
	for _, tag := range incoming {
		if scope, ok := tagrules.ScopeOf(tag); ok {
			incomingByScope[scope] = tag
		}
	}
	if len(incomingByScope) == 0 {
		return nil
	}

	result := make(map[string][]string)
	for hash, current := range currentByHash {
		var toRemove []string
		for _, tag := range splitTaskTags(current) {
			scope, ok := tagrules.ScopeOf(tag)
			if !ok {
				continue
			}
			if incomingValue, conflicts := incomingByScope[scope]; conflicts && incomingValue != tag {
				toRemove = append(toRemove, tag)
			}
		}
		if len(toRemove) > 0 {
			result[hash] = toRemove
		}
	}

	return result
}

func (s *Repository) SetCategory(hash string, category string) error {
	normalizedCategory, shouldRemove := normalizeTaskCategory(category)

	if shouldRemove {
		if err := s.client.RemoveCategory(hash); err != nil {
			return errors.Wrap(err, "failed to remove torrent category")
		}
		return nil
	}

	qbitCategory, err := s.matchExistingCategory(normalizedCategory)
	if err != nil {
		return errors.Wrap(err, "failed to resolve torrent category")
	}

	if err := s.client.SetCategory(hash, qbitCategory); err != nil {
		return errors.Wrap(err, "failed to set torrent category")
	}
	return nil
}

// matchExistingCategory returns the qBittorrent category spelling for a name.
// qBittorrent treats category names as case-sensitive when updating torrents,
// while the application matches category names case-insensitively.
func (s *Repository) matchExistingCategory(category string) (string, error) {
	categories, err := s.client.GetCategories()
	if err != nil {
		return "", err
	}

	return matchCategoryName(categories, category), nil
}

func matchCategoryName(categories map[string]qbt.Category, category string) string {
	for mapName, qbitCategory := range categories {
		candidate := qbitCategory.Name
		if candidate == "" {
			candidate = mapName
		}
		if strings.EqualFold(candidate, category) {
			return candidate
		}
	}

	return category
}

func normalizeTaskCategory(category string) (string, bool) {
	normalizedCategory := strings.TrimSpace(category)
	return normalizedCategory, normalizedCategory == ""
}

func (s *Repository) SetShareLimit(hash string, limits entities.TaskShareLimit) error {
	// Default to -2 (use global limit) if InactiveSeedingTimeLimit is not provided (zero value)
	inactiveSeedingTimeLimit := limits.InactiveSeedingTimeLimit
	if inactiveSeedingTimeLimit == 0 {
		inactiveSeedingTimeLimit = -2 // Use global limit by default
	}

	if err := s.client.SetTorrentShareLimit(hash, limits.RatioLimit, limits.SeedingTimeLimit, inactiveSeedingTimeLimit); err != nil {
		return errors.Wrap(err, "failed to set torrent share limit")
	}

	return nil
}

func (s *Repository) SetLocation(hash string, schema schemas.TaskSetLocationSchema) error {
	if err := s.client.SetTorrentLocation(hash, schema.Location); err != nil {
		return errors.Wrap(err, "failed to set torrent location")
	}

	return nil
}

func (s *Repository) Rename(hash string, schema schemas.TaskRenameSchema) error {
	if err := s.client.RenameTorrent(hash, schema.NewName); err != nil {
		return errors.Wrap(err, "failed to rename torrent")
	}

	return nil
}

func ParseMagnetLink(magnetURI string) (*entities.TaskMagnetLink, error) {
	uri, err := qbt.ParseMagnetLink(magnetURI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse magnet link")
	}
	if len(uri.Hash) == 32 {
		decoded, err := base32.StdEncoding.DecodeString(strings.ToUpper(uri.Hash))
		if err != nil {
			return nil, errors.Wrap(err, "invalid Base32 torrent hash")
		}
		uri.Hash = hex.EncodeToString(decoded)
	}

	return &entities.TaskMagnetLink{
		Hash:        uri.Hash,
		DisplayName: uri.DisplayName,
		Trackers:    uri.Trackers,
		ExactLength: uri.ExactLength,
		ExactSource: uri.ExactSource,
	}, nil
}

func (s *Repository) SetSuperSeeding(hash string, schema schemas.TaskSuperSeedingSchema) error {
	if err := s.client.SuperSeedingMode(hash, schema.Enabled); err != nil {
		return errors.Wrap(err, "failed to set super seeding mode")
	}

	return nil
}

func (s *Repository) ForceRecheck(hash string) error {
	if err := s.client.ForceRecheck(hash); err != nil {
		return errors.Wrap(err, "failed to force recheck torrent")
	}

	return nil
}

func (s *Repository) ForceReannounce(hash string) error {
	if err := s.client.ForceReannounce(hash); err != nil {
		return errors.Wrap(err, "failed to force reannounce torrent")
	}

	return nil
}

func (s *Repository) SetDownloadLimit(hash string, schema schemas.TaskSetDownloadLimitSchema) error {
	if err := s.client.SetTorrentDownloadLimit(hash, schema.Limit); err != nil {
		return errors.Wrap(err, "failed to set torrent download limit")
	}

	return nil
}

func (s *Repository) SetUploadLimit(hash string, schema schemas.TaskSetUploadLimitSchema) error {
	if err := s.client.SetTorrentUploadLimit(hash, schema.Limit); err != nil {
		return errors.Wrap(err, "failed to set torrent upload limit")
	}

	return nil
}

func (s *Repository) ListFiles(hash string) ([]*entities.TaskFile, error) {
	files, err := s.client.ListTorrentFiles(hash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list torrent files")
	}

	result := make([]*entities.TaskFile, len(files))
	for i, file := range files {
		result[i] = &entities.TaskFile{
			Index:        file.Index,
			Name:         file.Name,
			Size:         file.Size,
			Progress:     file.Progress,
			Priority:     file.Priority,
			IsSeed:       file.IsSeed,
			PieceRange:   file.PieceRange,
			Availability: file.Availability,
		}
	}

	return result, nil
}

func (s *Repository) SetFilePriority(hash string, schema schemas.TaskSetFilePrioritySchema) error {
	if err := s.client.SetFilePriority(hash, schema.Indexes, *schema.Priority); err != nil {
		return errors.Wrap(err, "failed to set torrent file priority")
	}
	return nil
}

func (s *Repository) ListTrackers(hash string) ([]*entities.TaskTracker, error) {
	trackers, err := s.client.GetTorrentTrackers(hash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list torrent trackers")
	}

	result := make([]*entities.TaskTracker, len(trackers))
	for i, t := range trackers {
		result[i] = &entities.TaskTracker{
			URL:           t.URL,
			Status:        t.Status,
			Tier:          t.Tier,
			NumPeers:      t.NumPeers,
			NumSeeds:      t.NumSeeds,
			NumLeeches:    t.NumLeeches,
			NumDownloaded: t.NumDownloaded,
			Message:       t.Msg,
		}
	}

	return result, nil
}

// AddTrackers adds one or more trackers to a torrent. Adding a tracker
// that's already present is a no-op on qBittorrent's side.
func (s *Repository) AddTrackers(hash string, urls []string) error {
	if err := s.client.AddTrackers(hash, urls); err != nil {
		return errors.Wrap(err, "failed to add trackers")
	}
	return nil
}

// RemoveTrackers removes one or more trackers from a torrent. Removing a
// tracker that isn't present is a no-op on qBittorrent's side.
func (s *Repository) RemoveTrackers(hash string, urls []string) error {
	if err := s.client.RemoveTrackers(hash, urls); err != nil {
		return errors.Wrap(err, "failed to remove trackers")
	}
	return nil
}

// EditTracker swaps a tracker's URL in place, preserving per-tracker state
// (like tier) that a RemoveTrackers+AddTrackers pair would lose.
func (s *Repository) EditTracker(hash, origURL, newURL string) error {
	if err := s.client.EditTracker(hash, origURL, newURL); err != nil {
		return errors.Wrap(err, "failed to edit tracker")
	}
	return nil
}

// SetQueuePriority reorders a torrent within qBittorrent's download queue.
// action must be one of "top", "up", "down", "bottom" - already validated by
// schemas.TaskSetQueuePrioritySchema's binding tag before reaching here.
func (s *Repository) SetQueuePriority(hash string, action string) error {
	var err error
	switch action {
	case "top":
		err = s.client.TopTorrentsPriority(hash)
	case "up":
		err = s.client.IncreaseTorrentsPriority(hash)
	case "down":
		err = s.client.DecreaseTorrentsPriority(hash)
	case "bottom":
		err = s.client.BottomTorrentsPriority(hash)
	default:
		return fmt.Errorf("unsupported queue priority action: %s", action)
	}
	if err != nil {
		return errors.Wrap(err, "failed to set torrent queue priority")
	}
	return nil
}

func (s *Repository) ListPeers(hash string) ([]*entities.TaskPeer, error) {
	peers, err := s.client.GetTorrentPeers(hash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list torrent peers")
	}

	result := make([]*entities.TaskPeer, len(peers))
	for i, p := range peers {
		result[i] = &entities.TaskPeer{
			IP:            p.IP,
			Port:          p.Port,
			Client:        p.Client,
			Flags:         p.Flags,
			FlagsDesc:     p.FlagsDesc,
			Connection:    p.Connection,
			Country:       p.Country,
			CountryCode:   p.CountryCode,
			Downloaded:    p.Downloaded,
			DownloadSpeed: p.DownloadSpeed,
			Progress:      p.Progress,
			Uploaded:      p.Uploaded,
			UploadSpeed:   p.UploadSpeed,
			Relevance:     p.Relevance,
		}
	}

	return result, nil
}

func (s *Repository) GetLimits(hash string) (*entities.TaskLimits, error) {
	dlLimit, err := s.client.GetTorrentDownloadLimit(hash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get torrent download limit")
	}

	upLimit, err := s.client.GetTorrentUploadLimit(hash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get torrent upload limit")
	}

	// Use GetTorrent (which uses /info endpoint) instead of GetTorrentProperties
	// because /properties doesn't return share limit fields
	torrent, err := s.client.GetTorrent(hash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get torrent info")
	}

	// Normalize fields: use max_ratio/max_seeding_time if ratio_limit/seeding_time_limit are 0
	ratioLimit := torrent.RatioLimit
	if ratioLimit == 0 && torrent.MaxRatio != 0 {
		ratioLimit = torrent.MaxRatio
	}

	seedingTimeLimit := torrent.SeedingTimeLimit
	if seedingTimeLimit == 0 && torrent.MaxSeedingTime != 0 {
		seedingTimeLimit = torrent.MaxSeedingTime
	}

	return &entities.TaskLimits{
		DownloadLimit:            dlLimit,
		UploadLimit:              upLimit,
		ShareLimit:               torrent.Ratio, // Use current ratio for backward compatibility
		RatioLimit:               ratioLimit,
		SeedingTimeLimit:         seedingTimeLimit,
		InactiveSeedingTimeLimit: torrent.InactiveSeedingTimeLimit,
	}, nil
}

func toTask(item *qbt.TorrentResponse) *entities.Task {
	status := entities.TaskStatuses[constants.UnknownStatus]
	if value, ok := entities.TaskStatuses[item.State]; ok {
		status = value
	}

	active := true
	if slices.Contains(entities.InactiveTaskStates, item.State) {
		active = false
	}

	var completedAt *time.Time
	if item.CompletionOn > 0 {
		completedTime := time.Unix(int64(item.CompletionOn), 0)
		completedAt = &completedTime
	}

	return &entities.Task{
		ID:             item.Hash,
		Name:           strings.TrimSpace(item.Name),
		Hash:           item.Hash,
		InfoHashV1:     item.InfoHashV1,
		InfoHashV2:     item.InfoHashV2,
		CreatedAt:      time.Unix(int64(item.AddedOn), 0),
		CompletedAt:    completedAt,
		Category:       item.Category,
		Path:           item.SavePath,
		State:          status,
		Size:           item.Size,
		CompletedBytes: item.Completed,
		Priority:       item.Priority,
		Ratio:          item.Ratio,
		Progress:       item.Progress * 100,
		Popularity:     item.Popularity,
		MagnetURI:      item.MagnetURI,
		Active:         active,
		MagnetLink: entities.TaskMagnetLink{
			Hash:        item.MagnetLink.Hash,
			DisplayName: item.MagnetLink.DisplayName,
			Trackers:    item.MagnetLink.Trackers,
			ExactLength: item.MagnetLink.ExactLength,
			ExactSource: item.MagnetLink.ExactSource,
		},
		Pairs: entities.TaskPairs{
			SwarmSeeders:  item.NumComplete,
			SwarmLeechers: item.NumIncomplete,
			Seeders:       item.NumSeeds,
			Leechers:      item.NumLeechs,
		},
		SuperSeeding: item.SuperSeeding,
		NumSeeds:     item.NumSeeds,
		Tags:         strings.Split(item.Tags, ","),
		Network: entities.TaskNetwork{
			Download: entities.TaskDownload{
				Speed:  item.Dlspeed,
				Amount: item.Downloaded,
			},
			Upload: entities.TaskUpload{
				Speed:  item.Upspeed,
				Amount: item.Uploaded,
			},
		},
	}
}
