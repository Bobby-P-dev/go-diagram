package repository

import (
	"database/sql"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

type PostgresRepository struct {
	db *sql.DB
}

var _ artifact.Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}
