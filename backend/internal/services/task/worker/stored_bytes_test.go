package task

import (
	"context"
	"testing"

	"github.com/jfxdev/gardarr/internal/entities"
)

func TestStoredBytesUsesCompletedContent(t *testing.T) {
	large, partial, empty, invalid := int64(5497558138880), int64(1234), int64(0), int64(-1)
	for _, tc := range []struct {
		name  string
		tasks []*entities.Task
		want  *int64
	}{
		{"empty worker", nil, &empty},
		{"complete preexisting and partial", []*entities.Task{
			{ID: "one", Size: 10000, Progress: 100, CompletedBytes: &large},
			{ID: "two", Size: 9999, Progress: 10, CompletedBytes: &partial, Network: entities.TaskNetwork{Download: entities.TaskDownload{Amount: 999999}}},
		}, ptrStored(large + partial)},
		{"known zero", []*entities.Task{{ID: "zero", Size: 100, CompletedBytes: &empty}}, &empty},
		{"missing field", []*entities.Task{{ID: "known", CompletedBytes: &partial}, {ID: "missing", Size: 100}}, nil},
		{"unknown sentinel", []*entities.Task{{ID: "invalid", CompletedBytes: &invalid}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRepository()
			var size int64
			for _, task := range tc.tasks {
				repo.tasks[task.ID] = task
				size += int64(task.Size)
			}
			stats, err := (&service{repository: repo}).GetTasksStats(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if stats.TotalDiskSize != size {
				t.Fatalf("legacy size changed: %d != %d", stats.TotalDiskSize, size)
			}
			if tc.want == nil {
				if stats.StoredBytes != nil {
					t.Fatalf("expected unknown, got %d", *stats.StoredBytes)
				}
			} else if stats.StoredBytes == nil || *stats.StoredBytes != *tc.want {
				t.Fatalf("stored bytes: %v, want %d", stats.StoredBytes, *tc.want)
			}
		})
	}
}

func ptrStored(value int64) *int64 { return &value }
