package postgres

import (
	"context"
	"time"

	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/uptrace/bun"
)

type OrganizationRepository struct {
	db *bun.DB
}

func NewOrganizationRepository(db *bun.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Create(ctx context.Context, org *storage.Organization) error {
	dbOrg := &DBOrganization{
		Name: org.Name,
		Slug: org.Slug,
		Tier: org.Tier,
	}

	_, err := r.db.NewInsert().Model(dbOrg).Exec(ctx)
	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"organizations_slug_key\" (SQLSTATE=23505)" {
			return storage.ErrAlreadyExists
		}
		return err
	}

	org.ID = dbOrg.ID
	return nil
}

func (r *OrganizationRepository) Get(ctx context.Context, id string) (*storage.Organization, error) {
	var dbOrg DBOrganization
	err := r.db.NewSelect().
		Model(&dbOrg).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return &storage.Organization{
		ID:            dbOrg.ID,
		Name:          dbOrg.Name,
		Slug:          dbOrg.Slug,
		Tier:          dbOrg.Tier,
		GitHubOrgID:   dbOrg.GitHubOrgID,
		GitHubOrgName: dbOrg.GitHubOrgName,
		CreatedAt:     dbOrg.CreatedAt,
		UpdatedAt:     dbOrg.UpdatedAt,
	}, nil
}

func (r *OrganizationRepository) Update(ctx context.Context, org *storage.Organization) error {
	dbOrg := &DBOrganization{
		ID:            org.ID,
		Name:          org.Name,
		Slug:          org.Slug,
		Tier:          org.Tier,
		GitHubOrgID:   org.GitHubOrgID,
		GitHubOrgName: org.GitHubOrgName,
		UpdatedAt:     time.Now(),
	}

	res, err := r.db.NewUpdate().
		Model(dbOrg).
		Column("name", "slug", "tier", "github_org_id", "github_org_name", "updated_at").
		Where("id = ?", org.ID).
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

	org.UpdatedAt = dbOrg.UpdatedAt
	return nil
}

func (r *OrganizationRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.NewUpdate().
		Model((*DBOrganization)(nil)).
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
