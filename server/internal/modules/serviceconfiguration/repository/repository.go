package repository

import (
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("service not found")
var ErrDuplicate = errors.New("service name already exists in workspace")

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }
