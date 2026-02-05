package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/uptrace/bun"
)

type organizationMemberRepository struct {
	db *bun.DB
}

func NewOrganizationMemberRepository(db *bun.DB) storage.OrganizationMemberRepository {
	return &organizationMemberRepository{db: db}
}

func (r *organizationMemberRepository) Create(ctx context.Context, member *storage.OrganizationMember) error {
	dbMember := &DBOrganizationMember{
		OrganizationID: member.OrganizationID,
		UserID:         member.UserID,
		Role:           UserRole(member.Role),
	}

	_, err := r.db.NewInsert().Model(dbMember).Returning("*").Exec(ctx)
	if err != nil {
		return err
	}

	member.ID = dbMember.ID
	member.CreatedAt = dbMember.CreatedAt
	member.UpdatedAt = dbMember.UpdatedAt
	return nil
}

func (r *organizationMemberRepository) GetByUserAndOrg(ctx context.Context, userID, orgID string) (*storage.OrganizationMember, error) {
	dbMember := new(DBOrganizationMember)
	err := r.db.NewSelect().
		Model(dbMember).
		Where("user_id = ?", userID).
		Where("organization_id = ?", orgID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.OrganizationMember{
		ID:             dbMember.ID,
		OrganizationID: dbMember.OrganizationID,
		UserID:         dbMember.UserID,
		Role:           storage.UserRole(dbMember.Role),
		CreatedAt:      dbMember.CreatedAt,
		UpdatedAt:      dbMember.UpdatedAt,
	}, nil
}

func (r *organizationMemberRepository) ListByUser(ctx context.Context, userID string) ([]*storage.OrganizationMember, error) {
	var dbMembers []*DBOrganizationMember
	err := r.db.NewSelect().
		Model(&dbMembers).
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	members := make([]*storage.OrganizationMember, len(dbMembers))
	for i, dbMember := range dbMembers {
		members[i] = &storage.OrganizationMember{
			ID:             dbMember.ID,
			OrganizationID: dbMember.OrganizationID,
			UserID:         dbMember.UserID,
			Role:           storage.UserRole(dbMember.Role),
			CreatedAt:      dbMember.CreatedAt,
			UpdatedAt:      dbMember.UpdatedAt,
		}
	}

	return members, nil
}

func (r *organizationMemberRepository) ListByOrganization(ctx context.Context, orgID string) ([]*storage.OrganizationMember, error) {
	var dbMembers []*DBOrganizationMember
	err := r.db.NewSelect().
		Model(&dbMembers).
		Where("organization_id = ?", orgID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	members := make([]*storage.OrganizationMember, len(dbMembers))
	for i, dbMember := range dbMembers {
		members[i] = &storage.OrganizationMember{
			ID:             dbMember.ID,
			OrganizationID: dbMember.OrganizationID,
			UserID:         dbMember.UserID,
			Role:           storage.UserRole(dbMember.Role),
			CreatedAt:      dbMember.CreatedAt,
			UpdatedAt:      dbMember.UpdatedAt,
		}
	}

	return members, nil
}

func (r *organizationMemberRepository) UpdateRole(ctx context.Context, id string, role storage.UserRole) error {
	res, err := r.db.NewUpdate().
		Model((*DBOrganizationMember)(nil)).
		Set("role = ?", role).
		Set("updated_at = ?", time.Now()).
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

func (r *organizationMemberRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.NewUpdate().
		Model((*DBOrganizationMember)(nil)).
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
