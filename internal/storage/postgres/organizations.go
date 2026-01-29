// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package postgres

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/regrada-ai/regrada-be/internal/storage"
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
	}, nil
}
