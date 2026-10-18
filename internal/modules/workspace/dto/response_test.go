package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
)

func TestResponsePreservesWorkspaceJSON(t *testing.T) {
	item := &model.Workspace{ID: uuid.New(), Name: "project", Description: "notes", RootPath: "/project", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	before, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(FromModel(item))
	if err != nil || string(before) != string(after) {
		t.Fatalf("response changed: %s -> %s (%v)", before, after, err)
	}
}
