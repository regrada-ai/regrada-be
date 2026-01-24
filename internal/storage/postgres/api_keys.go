package postgres

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type APIKeyRepository struct {
	db *bun.DB
}

func NewAPIKeyRepository(db *bun.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*storage.APIKey, error) {
	var dbKey DBAPIKey
	err := r.db.NewSelect().
		Model(&dbKey).
		Where("key_hash = ?", keyHash).
		Where("revoked_at IS NULL").
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return &storage.APIKey{
		ID:             dbKey.ID,
		OrganizationID: dbKey.OrganizationID,
		KeyHash:        dbKey.KeyHash,
		KeyPrefix:      dbKey.KeyPrefix,
		Name:           dbKey.Name,
		Tier:           dbKey.Tier,
		Scopes:         dbKey.Scopes,
		RateLimitRPM:   dbKey.RateLimitRPM,
		LastUsedAt:     dbKey.LastUsedAt,
		ExpiresAt:      dbKey.ExpiresAt,
		CreatedAt:      dbKey.CreatedAt,
		RevokedAt:      dbKey.RevokedAt,
	}, nil
}

func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*DBAPIKey)(nil)).
		Set("last_used_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}
