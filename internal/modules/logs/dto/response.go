package dto

import "github.com/prasannashrestha011/devOrbit/internal/logs"

type ListResponse struct {
	Entries     []logs.Entry     `json:"entries"`
	NextCursor  string           `json:"next_cursor"`
	Diagnostics logs.Diagnostics `json:"diagnostics"`
}

func NewListResponse(entries []logs.Entry, diagnostics logs.Diagnostics) ListResponse {
	if entries == nil {
		entries = []logs.Entry{}
	}
	var cursor string
	if len(entries) > 0 {
		cursor = entries[len(entries)-1].ID
	}
	return ListResponse{Entries: entries, NextCursor: cursor, Diagnostics: diagnostics}
}
