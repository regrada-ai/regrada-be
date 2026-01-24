package postgres

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type ProjectRepository struct {
	db *bun.DB
}

func NewProjectRepository(db *bun.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Get(ctx context.Context, id string) (*storage.Project, error) {
	var dbProject DBProject
	err := r.db.NewSelect().
		Model(&dbProject).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return &storage.Project{
		ID:             dbProject.ID,
		OrganizationID: dbProject.OrganizationID,
		Name:           dbProject.Name,
		Slug:           dbProject.Slug,
	}, nil
}
