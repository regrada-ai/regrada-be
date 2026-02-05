// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package postgres

import (
	"context"
	"time"

	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/uptrace/bun"
)

type ProjectRepository struct {
	db *bun.DB
}

func NewProjectRepository(db *bun.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, project *storage.Project) error {
	dbProject := &DBProject{
		OrganizationID: project.OrganizationID,
		Name:           project.Name,
		Slug:           project.Slug,
		DefaultBranch:  "main",
	}

	_, err := r.db.NewInsert().Model(dbProject).Exec(ctx)
	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"projects_organization_id_slug_key\" (SQLSTATE=23505)" {
			return storage.ErrAlreadyExists
		}
		return err
	}

	project.ID = dbProject.ID
	return nil
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

func (r *ProjectRepository) ListByOrganization(ctx context.Context, orgID string) ([]*storage.Project, error) {
	var dbProjects []DBProject
	err := r.db.NewSelect().
		Model(&dbProjects).
		Where("organization_id = ?", orgID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	projects := make([]*storage.Project, len(dbProjects))
	for i, dbProject := range dbProjects {
		projects[i] = &storage.Project{
			ID:             dbProject.ID,
			OrganizationID: dbProject.OrganizationID,
			Name:           dbProject.Name,
			Slug:           dbProject.Slug,
		}
	}

	return projects, nil
}

func (r *ProjectRepository) Update(ctx context.Context, project *storage.Project) error {
	dbProject := &DBProject{
		ID:   project.ID,
		Name: project.Name,
		Slug: project.Slug,
	}

	res, err := r.db.NewUpdate().
		Model(dbProject).
		Column("name", "slug", "updated_at").
		Set("updated_at = ?", time.Now()).
		Where("id = ?", project.ID).
		Where("deleted_at IS NULL").
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.NewUpdate().
		Model((*DBProject)(nil)).
		Set("deleted_at = ?", time.Now()).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}
