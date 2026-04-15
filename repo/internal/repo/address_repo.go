package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AddressRepo struct {
	pool *pgxpool.Pool
}

func NewAddressRepo(pool *pgxpool.Pool) *AddressRepo {
	return &AddressRepo{pool: pool}
}

func (r *AddressRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.DeliveryAddress, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, label, recipient_name, phone, line1, line2, city, state,
		       postal_code, country_code, is_default, created_at, updated_at
		FROM delivery_addresses
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY is_default DESC, created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	var addrs []model.DeliveryAddress
	for rows.Next() {
		var a model.DeliveryAddress
		if err := rows.Scan(&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone,
			&a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.CountryCode,
			&a.IsDefault, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}
		addrs = append(addrs, a)
	}
	return addrs, rows.Err()
}

func (r *AddressRepo) GetByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*model.DeliveryAddress, error) {
	a := &model.DeliveryAddress{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, label, recipient_name, phone, line1, line2, city, state,
		       postal_code, country_code, is_default, created_at, updated_at
		FROM delivery_addresses
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID,
	).Scan(&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone,
		&a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.CountryCode,
		&a.IsDefault, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get address: %w", err)
	}
	return a, nil
}

func (r *AddressRepo) Create(ctx context.Context, a *model.DeliveryAddress) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// If setting as default, unset other defaults
	if a.IsDefault {
		_, err = tx.Exec(ctx, `
			UPDATE delivery_addresses SET is_default = false, updated_at = now()
			WHERE user_id = $1 AND is_default = true AND deleted_at IS NULL`, a.UserID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO delivery_addresses (id, user_id, label, recipient_name, phone, line1, line2,
			city, state, postal_code, country_code, is_default, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		a.ID, a.UserID, a.Label, a.RecipientName, a.Phone, a.Line1, a.Line2,
		a.City, a.State, a.PostalCode, a.CountryCode, a.IsDefault, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert address: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *AddressRepo) Update(ctx context.Context, a *model.DeliveryAddress) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if a.IsDefault {
		_, err = tx.Exec(ctx, `
			UPDATE delivery_addresses SET is_default = false, updated_at = now()
			WHERE user_id = $1 AND id != $2 AND is_default = true AND deleted_at IS NULL`,
			a.UserID, a.ID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE delivery_addresses SET label=$2, recipient_name=$3, phone=$4, line1=$5, line2=$6,
			city=$7, state=$8, postal_code=$9, country_code=$10, is_default=$11, updated_at=$12
		WHERE id = $1 AND deleted_at IS NULL`,
		a.ID, a.Label, a.RecipientName, a.Phone, a.Line1, a.Line2,
		a.City, a.State, a.PostalCode, a.CountryCode, a.IsDefault, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update address: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *AddressRepo) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE delivery_addresses SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("address not found")
	}
	return nil
}
