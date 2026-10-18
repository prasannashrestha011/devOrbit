package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
)

func TestResponsePreservesServiceJSON(t *testing.T) {
	enabled := false
	item := &model.Service{
		ID: uuid.New(), WorkspaceID: uuid.New(), Name: "worker", Command: "go",
		Args: []string{"run"}, WorkingDirectory: "src", Env: map[string]string{"KEY": "value"},
		Enabled: &enabled, AutoRestart: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	before, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(FromModel(item))
	if err != nil || string(before) != string(after) {
		t.Fatalf("response changed: %s -> %s (%v)", before, after, err)
	}
}
