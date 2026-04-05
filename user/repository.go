package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for users and devices.
type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByProviderID(ctx context.Context, providerID string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListDevices(ctx context.Context, userID uuid.UUID) ([]*Device, error)
	CreateDevice(ctx context.Context, device *Device) error
	DeleteDevice(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, provider_id, email, name, date_of_birth, avatar_url, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	)
	u, err := scanUser(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user %s: %w", id, err)
	}
	return u, nil
}

func (r *repository) FindByProviderID(ctx context.Context, providerID string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, provider_id, email, name, date_of_birth, avatar_url, created_at, updated_at
		 FROM users WHERE provider_id = $1`, providerID,
	)
	u, err := scanUser(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by provider_id %s: %w", providerID, err)
	}
	return u, nil
}

func (r *repository) Create(ctx context.Context, user *User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (provider_id, email, name, date_of_birth)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		user.ProviderID, user.Email, user.Name, user.DateOfBirth,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *repository) Update(ctx context.Context, user *User) error {
	err := r.pool.QueryRow(ctx,
		`UPDATE users
		 SET name = COALESCE($2, name),
		     date_of_birth = COALESCE($3, date_of_birth),
		     avatar_url = COALESCE($4, avatar_url),
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING updated_at`,
		user.ID, user.Name, user.DateOfBirth, user.AvatarURL,
	).Scan(&user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update user %s: %w", user.ID, err)
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", id, err)
	}
	return nil
}

func (r *repository) ListDevices(ctx context.Context, userID uuid.UUID) ([]*Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, type, created_at FROM devices WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Type, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, &d)
	}
	return devices, rows.Err()
}

func (r *repository) CreateDevice(ctx context.Context, device *Device) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO devices (user_id, name, type) VALUES ($1, $2, $3) RETURNING id, created_at`,
		device.UserID, device.Name, device.Type,
	).Scan(&device.ID, &device.CreatedAt)
	if err != nil {
		return fmt.Errorf("create device: %w", err)
	}
	return nil
}

func (r *repository) DeleteDevice(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM devices WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	return nil
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.ProviderID, &u.Email, &u.Name,
		&u.DateOfBirth, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	return &u, err
}
