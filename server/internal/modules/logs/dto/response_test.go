package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/prasannashrestha011/devOrbit/internal/logs"
)

func TestListResponseShape(t *testing.T) {
	empty, err := json.Marshal(NewListResponse(nil, logs.Diagnostics{}))
	if err != nil || !strings.Contains(string(empty), `"entries":[]`) || !strings.Contains(string(empty), `"next_cursor":""`) {
		t.Fatalf("empty response: %s (%v)", empty, err)
	}
	result := NewListResponse([]logs.Entry{{ID: "first"}, {ID: "last"}}, logs.Diagnostics{})
	if result.NextCursor != "last" {
		t.Fatalf("cursor = %q", result.NextCursor)
	}
}
