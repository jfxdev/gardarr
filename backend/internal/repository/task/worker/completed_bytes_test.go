package task

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestListPreservesCompletedBytesFromQbittorrent(t *testing.T) {
	const hash = "5a0cdacbd5571efcdd5e279fcb0e913b6c36cb36"
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/torrents/info" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"hash": hash, "magnet_uri": "magnet:?xt=urn:btih:" + hash, "completed": int64(5497558138880), "downloaded": 0},
				{"hash": hash, "magnet_uri": "magnet:?xt=urn:btih:" + hash, "completed": 0},
				{"hash": hash, "magnet_uri": "magnet:?xt=urn:btih:" + hash},
			})
		}
	})
	tasks, err := New(client).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 || tasks[0].CompletedBytes == nil || *tasks[0].CompletedBytes != 5497558138880 || tasks[0].Network.Download.Amount != 0 {
		t.Fatalf("lost seeded content: %#v", tasks)
	}
	if tasks[1].CompletedBytes == nil || *tasks[1].CompletedBytes != 0 || tasks[2].CompletedBytes != nil {
		t.Fatal("missing field was not distinguished from zero")
	}
}
