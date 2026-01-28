package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/uptrace/bun"
)

type userRepository struct {
	db *bun.DB
}

func NewUserRepository(db *bun.DB) storage.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *storage.User) error {
	dbUser := &DBUser{
		ID:        user.ID,
		Email:     user.Email,
		IDPSub:    user.IDPSub,
		Name:      user.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.db.NewInsert().Model(dbUser).Exec(ctx)
	if err != nil {
		return err
	}

	user.ID = dbUser.ID
	user.CreatedAt = dbUser.CreatedAt
	user.UpdatedAt = dbUser.UpdatedAt
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*storage.User, error) {
	dbUser := new(DBUser)
	err := r.db.NewSelect().
		Model(dbUser).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.User{
		ID:        dbUser.ID,
		Email:     dbUser.Email,
		IDPSub:    dbUser.IDPSub,
		Name:      dbUser.Name,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*storage.User, error) {
	dbUser := new(DBUser)
	err := r.db.NewSelect().
		Model(dbUser).
		Where("email = ?", email).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.User{
		ID:        dbUser.ID,
		Email:     dbUser.Email,
		IDPSub:    dbUser.IDPSub,
		Name:      dbUser.Name,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}, nil
}

func (r *userRepository) GetByIDPSub(ctx context.Context, idpSub string) (*storage.User, error) {
	dbUser := new(DBUser)
	err := r.db.NewSelect().
		Model(dbUser).
		Where("idp_sub = ?", idpSub).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.User{
		ID:        dbUser.ID,
		Email:     dbUser.Email,
		IDPSub:    dbUser.IDPSub,
		Name:      dbUser.Name,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}, nil
}

func (r *userRepository) Update(ctx context.Context, user *storage.User) error {
	dbUser := &DBUser{
		ID:        user.ID,
		Email:     user.Email,
		IDPSub:    user.IDPSub,
		Name:      user.Name,
		UpdatedAt: time.Now(),
	}

	res, err := r.db.NewUpdate().
		Model(dbUser).
		Column("email", "name", "updated_at").
		Where("id = ?", user.ID).
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

	user.UpdatedAt = dbUser.UpdatedAt
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.NewUpdate().
		Model((*DBUser)(nil)).
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
