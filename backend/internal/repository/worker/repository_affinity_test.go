package worker

import (
	"testing"

	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
)

func TestDeleteWorkerClearsCategoryPreferencesAtomically(t *testing.T) {
	for _, failDelete := range []bool{false, true} {
		name := "success"
		if failDelete {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			db := database.SetupTestDB(t, &models.Worker{}, &models.Category{})
			worker := models.Worker{Name: "Preferred", Address: "http://offline.invalid"}
			other := models.Worker{Name: "Other", Address: "http://other.invalid"}
			for _, w := range []*models.Worker{&worker, &other} {
				if err := db.DB.Create(w).Error; err != nil {
					t.Fatal(err)
				}
			}
			categories := []models.Category{
				{ID: "affected", Name: "Affected", PreferredWorkerID: worker.UUID.String()},
				{ID: "unrelated", Name: "Unrelated", PreferredWorkerID: other.UUID.String()},
			}
			for i := range categories {
				if err := db.DB.Create(&categories[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			if failDelete {
				if err := db.DB.Exec("CREATE TRIGGER reject_worker_delete BEFORE DELETE ON workers BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := &Repository{db: db}
			err := repo.DeleteWorker(worker.UUID)
			if failDelete && err == nil {
				t.Fatal("expected delete failure")
			}
			if !failDelete && err != nil {
				t.Fatal(err)
			}
			var affected, unrelated models.Category
			if err := db.DB.First(&affected, "id = ?", "affected").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.DB.First(&unrelated, "id = ?", "unrelated").Error; err != nil {
				t.Fatal(err)
			}
			want := ""
			if failDelete {
				want = worker.UUID.String()
			}
			if affected.PreferredWorkerID != want {
				t.Fatalf("expected %q, got %q", want, affected.PreferredWorkerID)
			}
			if unrelated.PreferredWorkerID != other.UUID.String() {
				t.Fatal("unrelated preference changed")
			}
			var count int64
			if err := db.DB.Model(&models.Worker{}).Where("uuid = ?", worker.UUID).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if (count == 1) != failDelete {
				t.Fatal("unexpected worker persistence")
			}
		})
	}
}
