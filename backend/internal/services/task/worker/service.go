package task

import (
	"context"
	"sort"
	"strings"

	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/interfaces"
	repository "github.com/jfxdev/gardarr/internal/repository/task/worker"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/pkg/torrentfile"
	"github.com/jfxdev/go-qbt"
)

type service struct {
	repository interfaces.TaskRepositoryInterface
}

// wordSplitReplacer replaces common punctuation/separators with spaces for tokenization
var wordSplitReplacer = strings.NewReplacer(
	".", " ", "_", " ", "-", " ", "(", " ", ")", " ", "[", " ", "]", " ",
	"{", " ", "}", " ", ":", " ", ";", " ", ",", " ", "!", " ", "?", " ",
	"@", " ", "#", " ", "$", " ", "%", " ", "^", " ", "&", " ", "*", " ",
	"+", " ", "=", " ", "|", " ", "\\", " ", "/", " ", "<", " ", ">", " ",
	"~", " ", "`", " ", "'", " ", "\"", " ",
)

func New(client *qbt.Client) interfaces.TaskService {
	return &service{
		repository: repository.New(client),
	}
}

func (s *service) ListTasks(ctx context.Context) ([]*entities.Task, error) {
	return s.repository.List()
}

func (s *service) GetTask(ctx context.Context, id string) (*entities.Task, error) {
	return s.repository.Get(id)
}

func (s *service) GetTaskLimits(ctx context.Context, id string) (*entities.TaskLimits, error) {
	return s.repository.GetLimits(id)
}

func (s *service) CreateTask(ctx context.Context, schema schemas.TaskCreateSchema) (*entities.Task, error) {
	uri, err := repository.ParseMagnetLink(schema.MagnetURI)
	if err != nil {
		return nil, err
	}

	list, err := s.repository.List()
	if err != nil {
		return nil, err
	}

	for _, item := range list {
		if strings.EqualFold(item.Hash, uri.Hash) || strings.EqualFold(item.MagnetLink.Hash, uri.Hash) {
			item.WasCreated = false
			return item, nil
		}
	}

	task, err := s.repository.Add(schema)
	if err == nil && task != nil {
		task.WasCreated = true
	}
	return task, err
}

// CreateTaskFromFile adds a torrent from an uploaded .torrent file. Dedupe by
// info-hash mirrors CreateTask's magnet flow.
func (s *service) CreateTaskFromFile(ctx context.Context, fileName string, fileData []byte, schema schemas.TaskCreateFromFileSchema) (*entities.Task, error) {
	hashes, err := torrentfile.InfoHashes(fileData)
	if err != nil {
		return nil, err
	}

	list, err := s.repository.List()
	if err != nil {
		return nil, err
	}

	for _, item := range list {
		if torrentfile.MatchesInfoHash(hashes, item.InfoHashV1, item.InfoHashV2) {
			item.WasCreated = false
			return item, nil
		}
	}

	task, err := s.repository.AddFile(fileName, fileData, hashes, schema)
	if err == nil && task != nil {
		task.WasCreated = true
	}
	return task, err
}

func (s *service) StopTask(ctx context.Context, hash string) error {
	return s.repository.Stop(hash)
}

func (s *service) DeleteTask(ctx context.Context, id string, deleteFiles bool) error {
	return s.repository.Delete(id, deleteFiles)
}

func (s *service) StartTask(ctx context.Context, hash string) error {
	return s.repository.Start(hash)
}

func (s *service) ForceResumeTask(ctx context.Context, hash string) error {
	return s.repository.ForceResume(hash)
}

func (s *service) SetTaskLocation(ctx context.Context, hash string, schema schemas.TaskSetLocationSchema) error {
	return s.repository.SetLocation(hash, schema)
}

func (s *service) RenameTask(ctx context.Context, hash string, schema schemas.TaskRenameSchema) error {
	return s.repository.Rename(hash, schema)
}

func (s *service) SetTaskSuperSeeding(ctx context.Context, hash string, schema schemas.TaskSuperSeedingSchema) error {
	return s.repository.SetSuperSeeding(hash, schema)
}

func (s *service) ForceRecheckTask(ctx context.Context, hash string) error {
	return s.repository.ForceRecheck(hash)
}

func (s *service) ForceReannounceTask(ctx context.Context, hash string) error {
	return s.repository.ForceReannounce(hash)
}

func (s *service) SetTaskDownloadLimit(ctx context.Context, hash string, schema schemas.TaskSetDownloadLimitSchema) error {
	return s.repository.SetDownloadLimit(hash, schema)
}

func (s *service) SetTaskUploadLimit(ctx context.Context, hash string, schema schemas.TaskSetUploadLimitSchema) error {
	return s.repository.SetUploadLimit(hash, schema)
}

func (s *service) SetTaskTags(ctx context.Context, hash string, schema schemas.TaskSetTagsSchema) error {
	return s.repository.SetTags(hash, schema.Tags)
}

func (s *service) AddTaskTags(ctx context.Context, hash string, tags []string) error {
	return s.repository.AddTags(hash, tags)
}

func (s *service) SetTaskCategory(ctx context.Context, hash string, schema schemas.TaskSetCategorySchema) error {
	return s.repository.SetCategory(hash, schema.Category)
}

func (s *service) ListTaskFiles(ctx context.Context, hash string) ([]*entities.TaskFile, error) {
	return s.repository.ListFiles(hash)
}

func (s *service) SetFilePriority(ctx context.Context, hash string, schema schemas.TaskSetFilePrioritySchema) error {
	return s.repository.SetFilePriority(hash, schema)
}

func (s *service) ListTaskTrackers(ctx context.Context, hash string) ([]*entities.TaskTracker, error) {
	return s.repository.ListTrackers(hash)
}

func (s *service) AddTaskTrackers(ctx context.Context, hash string, urls []string) error {
	return s.repository.AddTrackers(hash, urls)
}

func (s *service) RemoveTaskTrackers(ctx context.Context, hash string, urls []string) error {
	return s.repository.RemoveTrackers(hash, urls)
}

func (s *service) EditTaskTracker(ctx context.Context, hash, origURL, newURL string) error {
	return s.repository.EditTracker(hash, origURL, newURL)
}

func (s *service) SetTaskQueuePriority(ctx context.Context, hash string, schema schemas.TaskSetQueuePrioritySchema) error {
	return s.repository.SetQueuePriority(hash, schema.Action)
}

func (s *service) ListTaskPeers(ctx context.Context, hash string) ([]*entities.TaskPeer, error) {
	return s.repository.ListPeers(hash)
}

func (s *service) GetTasksStats(ctx context.Context) (*entities.TaskStats, error) {
	tasks, err := s.repository.List()
	if err != nil {
		return nil, err
	}

	var storedBytes int64
	stats := &entities.TaskStats{
		StoredBytes:          &storedBytes,
		TotalDiskSize:        0,
		CurrentUploadSpeed:   0,
		CurrentDownloadSpeed: 0,
		AverageRatio:         0,
		MedianRatio:          0,
		HighestRatio:         0,
		LowestRatio:          0,
		ActiveTasksCount:     0,
		TotalTasksCount:      len(tasks),
		ActiveSeeds:          0,
		ActivePeers:          0,
		SwarmSeeders:         0,
		SwarmLeechers:        0,
		CategoryUsage:        make(map[string]int),
		TagsUsage:            make(map[string]int),
		WordCloud:            calculateWordCloud(tasks),
	}

	if len(tasks) == 0 {
		return stats, nil
	}

	var totalRatio float64
	var ratios []float64
	var totalUploadSpeed, totalDownloadSpeed int

	for _, task := range tasks {
		// Calculate total disk size
		stats.TotalDiskSize += int64(task.Size)
		// completed is content present, whereas downloaded is a traffic counter
		// and Size includes data that has not been downloaded yet.
		if task.CompletedBytes == nil || *task.CompletedBytes < 0 {
			stats.StoredBytes = nil
		} else if stats.StoredBytes != nil {
			storedBytes += *task.CompletedBytes
		}

		// Calculate current speeds
		totalUploadSpeed += task.Network.Upload.Speed
		totalDownloadSpeed += task.Network.Download.Speed

		// Consider task active either by explicit flag or by well-known active states
		if task.Active || strings.EqualFold(task.State, "DOWNLOADING") || strings.EqualFold(task.State, "SEEDING") || strings.EqualFold(task.State, "UPLOADING") || strings.EqualFold(task.State, "FORCED_DOWNLOAD") || strings.EqualFold(task.State, "FORCED_UPLOAD") {
			stats.ActiveTasksCount++
		}

		// Count active seeds and peers
		stats.ActiveSeeds += task.Pairs.Seeders
		stats.ActivePeers += task.Pairs.Leechers

		stats.SwarmSeeders += task.Pairs.SwarmSeeders
		stats.SwarmLeechers += task.Pairs.SwarmLeechers

		// Calculate ratios for average and median
		totalRatio += task.Ratio
		ratios = append(ratios, task.Ratio)

		// Count category usage
		if task.Category != "" {
			stats.CategoryUsage[task.Category]++
		}

		// Count tags usage
		for _, tag := range task.Tags {
			if tag != "" {
				stats.TagsUsage[tag]++
			}
		}
	}

	// Calculate average ratio
	stats.AverageRatio = totalRatio / float64(len(tasks))

	// Calculate median ratio
	sort.Float64s(ratios)
	if len(ratios) > 0 {
		if len(ratios)%2 == 0 {
			stats.MedianRatio = (ratios[len(ratios)/2-1] + ratios[len(ratios)/2]) / 2
		} else {
			stats.MedianRatio = ratios[len(ratios)/2]
		}

		// Calculate highest and lowest ratios
		stats.HighestRatio = ratios[len(ratios)-1]
		stats.LowestRatio = ratios[0]
	}

	// Set current speeds
	stats.CurrentUploadSpeed = totalUploadSpeed
	stats.CurrentDownloadSpeed = totalDownloadSpeed

	return stats, nil
}

func (s *service) SetTaskShareLimit(ctx context.Context, id string, schema schemas.TaskSetShareLimitSchema) error {
	return s.repository.SetShareLimit(id, entities.TaskShareLimit{
		RatioLimit:               schema.RatioLimit,
		SeedingTimeLimit:         schema.SeedingTimeLimit,
		InactiveSeedingTimeLimit: schema.InactiveSeedingTimeLimit,
	})
}

// calculateWordCloud extracts and counts the top 25 most used terms in task names
func calculateWordCloud(tasks []*entities.Task) map[string]int {
	wordCount := make(map[string]int)

	// Common words to ignore (stop words)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "can": true, "this": true, "that": true,
		"these": true, "those": true, "i": true, "you": true, "he": true, "she": true,
		"it": true, "we": true, "they": true, "me": true, "him": true, "her": true,
		"us": true, "them": true, "my": true, "your": true, "his": true, "its": true,
		"our": true, "their": true, "from": true, "up": true, "down": true,
		"out": true, "off": true, "over": true, "under": true, "again": true, "further": true,
		"then": true, "once": true, "here": true, "there": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "any": true, "both": true, "each": true,
		"few": true, "more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "nor": true, "not": true, "only": true, "own": true, "same": true,
		"so": true, "than": true, "too": true, "very": true, "s": true, "t": true,
		"just": true, "don": true, "now": true,
		"d": true, "ll": true, "m": true, "o": true, "re": true, "ve": true, "y": true,
		"ain": true, "aren": true, "couldn": true, "didn": true, "doesn": true, "hadn": true,
		"hasn": true, "haven": true, "isn": true, "ma": true, "mightn": true, "mustn": true,
		"needn": true, "shan": true, "shouldn": true, "wasn": true, "weren": true, "won": true,
		"wouldn": true, "download": true, "upload": true, "seed": true, "leech": true, "peer": true,
	}

	// Extract words from each task name
	for _, task := range tasks {
		if task.Name == "" {
			continue
		}

		// Normalize and replace common delimiters with spaces using a reusable replacer
		text := strings.ToLower(task.Name)
		text = wordSplitReplacer.Replace(text)

		// Split into words
		words := strings.Fields(text)

		// Count each word (skip stop words and very short words)
		for _, word := range words {
			// Clean the word (remove any remaining non-alphanumeric characters)
			word = strings.TrimSpace(word)
			if len(word) < 3 || stopWords[word] {
				continue
			}

			// Only count words that contain at least one letter
			hasLetter := false
			for _, char := range word {
				if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
					hasLetter = true
					break
				}
			}

			if hasLetter {
				wordCount[word]++
			}
		}
	}

	// Convert to slice for sorting
	type wordCountPair struct {
		word  string
		count int
	}

	var pairs []wordCountPair
	for word, count := range wordCount {
		pairs = append(pairs, wordCountPair{word, count})
	}

	// Sort by count (descending) and then by word (ascending) for consistency
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})

	// Take top 25 and convert back to map
	result := make(map[string]int)
	maxCount := 25
	if len(pairs) < maxCount {
		maxCount = len(pairs)
	}

	for i := 0; i < maxCount; i++ {
		result[pairs[i].word] = pairs[i].count
	}

	return result
}
