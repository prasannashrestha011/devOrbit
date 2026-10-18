package database

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestContextDatabaseResolution(t *testing.T) {
	if _, err := FromContext(context.Background()); err == nil {
		t.Fatal("missing database accepted")
	}
	primary, transaction := &gorm.DB{}, &gorm.DB{}
	ctx := WithDB(context.Background(), primary)
	if got, err := FromContext(ctx); err != nil || got != primary {
		t.Fatalf("primary database resolution: %v", err)
	}
	ctx = WithDB(ctx, transaction)
	if got, err := FromContext(ctx); err != nil || got != transaction {
		t.Fatalf("transaction did not override database: %v", err)
	}
}
