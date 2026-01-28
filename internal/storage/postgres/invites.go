package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/uptrace/bun"
)

type inviteRepository struct {
	db *bun.DB
}

func NewInviteRepository(db *bun.DB) storage.InviteRepository {
	return &inviteRepository{db: db}
}

func (r *inviteRepository) Create(ctx context.Context, invite *storage.Invite) error {
	dbInvite := &DBInvite{
		ID:             invite.ID,
		OrganizationID: invite.OrganizationID,
		Email:          invite.Email,
		Role:           UserRole(invite.Role),
		Token:          invite.Token,
		InvitedBy:      invite.InvitedBy,
		ExpiresAt:      invite.ExpiresAt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err := r.db.NewInsert().Model(dbInvite).Exec(ctx)
	if err != nil {
		return err
	}

	invite.ID = dbInvite.ID
	invite.CreatedAt = dbInvite.CreatedAt
	invite.UpdatedAt = dbInvite.UpdatedAt
	return nil
}

func (r *inviteRepository) GetByToken(ctx context.Context, token string) (*storage.Invite, error) {
	dbInvite := new(DBInvite)
	err := r.db.NewSelect().
		Model(dbInvite).
		Where("token = ?", token).
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.Invite{
		ID:             dbInvite.ID,
		OrganizationID: dbInvite.OrganizationID,
		Email:          dbInvite.Email,
		Role:           storage.UserRole(dbInvite.Role),
		Token:          dbInvite.Token,
		InvitedBy:      dbInvite.InvitedBy,
		AcceptedAt:     dbInvite.AcceptedAt,
		AcceptedBy:     dbInvite.AcceptedBy,
		RevokedAt:      dbInvite.RevokedAt,
		ExpiresAt:      dbInvite.ExpiresAt,
		CreatedAt:      dbInvite.CreatedAt,
		UpdatedAt:      dbInvite.UpdatedAt,
	}, nil
}

func (r *inviteRepository) GetByEmailAndOrg(ctx context.Context, email, orgID string) (*storage.Invite, error) {
	dbInvite := new(DBInvite)
	err := r.db.NewSelect().
		Model(dbInvite).
		Where("email = ?", email).
		Where("organization_id = ?", orgID).
		Where("accepted_at IS NULL").
		Where("revoked_at IS NULL").
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.Invite{
		ID:             dbInvite.ID,
		OrganizationID: dbInvite.OrganizationID,
		Email:          dbInvite.Email,
		Role:           storage.UserRole(dbInvite.Role),
		Token:          dbInvite.Token,
		InvitedBy:      dbInvite.InvitedBy,
		AcceptedAt:     dbInvite.AcceptedAt,
		AcceptedBy:     dbInvite.AcceptedBy,
		RevokedAt:      dbInvite.RevokedAt,
		ExpiresAt:      dbInvite.ExpiresAt,
		CreatedAt:      dbInvite.CreatedAt,
		UpdatedAt:      dbInvite.UpdatedAt,
	}, nil
}

func (r *inviteRepository) ListByOrganization(ctx context.Context, orgID string) ([]*storage.Invite, error) {
	var dbInvites []*DBInvite
	err := r.db.NewSelect().
		Model(&dbInvites).
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	invites := make([]*storage.Invite, len(dbInvites))
	for i, dbInvite := range dbInvites {
		invites[i] = &storage.Invite{
			ID:             dbInvite.ID,
			OrganizationID: dbInvite.OrganizationID,
			Email:          dbInvite.Email,
			Role:           storage.UserRole(dbInvite.Role),
			Token:          dbInvite.Token,
			InvitedBy:      dbInvite.InvitedBy,
			AcceptedAt:     dbInvite.AcceptedAt,
			AcceptedBy:     dbInvite.AcceptedBy,
			RevokedAt:      dbInvite.RevokedAt,
			ExpiresAt:      dbInvite.ExpiresAt,
			CreatedAt:      dbInvite.CreatedAt,
			UpdatedAt:      dbInvite.UpdatedAt,
		}
	}

	return invites, nil
}

func (r *inviteRepository) Accept(ctx context.Context, token, userID string) error {
	now := time.Now()
	res, err := r.db.NewUpdate().
		Model((*DBInvite)(nil)).
		Set("accepted_at = ?", now).
		Set("accepted_by = ?", userID).
		Set("updated_at = ?", now).
		Where("token = ?", token).
		Where("accepted_at IS NULL").
		Where("revoked_at IS NULL").
		Where("expires_at > ?", now).
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

func (r *inviteRepository) Revoke(ctx context.Context, id string) error {
	now := time.Now()
	res, err := r.db.NewUpdate().
		Model((*DBInvite)(nil)).
		Set("revoked_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Where("accepted_at IS NULL").
		Where("revoked_at IS NULL").
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
