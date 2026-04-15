package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogRepo struct {
	pool *pgxpool.Pool
}

func NewCatalogRepo(pool *pgxpool.Pool) *CatalogRepo {
	return &CatalogRepo{pool: pool}
}

func (r *CatalogRepo) Pool() *pgxpool.Pool { return r.pool }

// --- Program Sessions ---

type SessionFilter struct {
	Query    string
	Status   string
	Category string
	Limit    int
	Offset   int
}

func (r *CatalogRepo) ListSessions(ctx context.Context, f SessionFilter) ([]model.SessionWithAvailability, int, error) {
	where := "WHERE ps.deleted_at IS NULL"
	var args []interface{}
	argIdx := 1

	if f.Status != "" {
		where += fmt.Sprintf(" AND ps.status = $%d", argIdx)
		args = append(args, f.Status)
		argIdx++
	}
	if f.Category != "" {
		where += fmt.Sprintf(" AND ps.category = $%d", argIdx)
		args = append(args, f.Category)
		argIdx++
	}

	orderBy := "ORDER BY ps.start_at ASC, ps.id ASC"
	if f.Query != "" {
		where += fmt.Sprintf(` AND to_tsvector('english', coalesce(ps.title,'') || ' ' || coalesce(ps.short_description,'') || ' ' || coalesce(ps.category,'') || ' ' || coalesce(ps.instructor_name,''))
			@@ plainto_tsquery('english', $%d)`, argIdx)
		args = append(args, f.Query)
		orderBy = fmt.Sprintf(`ORDER BY ts_rank(
			to_tsvector('english', coalesce(ps.title,'') || ' ' || coalesce(ps.short_description,'') || ' ' || coalesce(ps.category,'') || ' ' || coalesce(ps.instructor_name,'')),
			plainto_tsquery('english', $%d)
		) DESC, ps.start_at ASC, ps.id ASC`, argIdx)
		argIdx++
	}

	baseQuery := fmt.Sprintf(`FROM program_sessions ps
		LEFT JOIN session_seat_inventory ssi ON ssi.session_id = ps.id
		%s`, where)

	// Count
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) "+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}

	// Fetch
	selectQ := fmt.Sprintf(`SELECT ps.id, ps.title, ps.description, ps.short_description, ps.category,
		ps.instructor_name, ps.tags, ps.start_at, ps.end_at, ps.seat_capacity,
		ps.price_minor_units, ps.currency, ps.registration_open_at, ps.registration_close_at,
		ps.requires_approval, ps.allows_waitlist, ps.status, ps.location, ps.created_by,
		ps.created_at, ps.updated_at,
		COALESCE(ssi.total_seats, ps.seat_capacity),
		COALESCE(ssi.reserved_seats, 0),
		COALESCE(ssi.available_seats, ps.seat_capacity)
		%s %s LIMIT $%d OFFSET $%d`, baseQuery, orderBy, argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []model.SessionWithAvailability
	for rows.Next() {
		var s model.SessionWithAvailability
		if err := rows.Scan(
			&s.ID, &s.Title, &s.Description, &s.ShortDescription, &s.Category,
			&s.InstructorName, &s.Tags, &s.StartAt, &s.EndAt, &s.SeatCapacity,
			&s.PriceMinorUnits, &s.Currency, &s.RegistrationOpenAt, &s.RegistrationCloseAt,
			&s.RequiresApproval, &s.AllowsWaitlist, &s.Status, &s.Location, &s.CreatedBy,
			&s.CreatedAt, &s.UpdatedAt,
			&s.TotalSeats, &s.ReservedSeats, &s.AvailableSeats,
		); err != nil {
			return nil, 0, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, total, rows.Err()
}

func (r *CatalogRepo) GetSessionByID(ctx context.Context, id uuid.UUID) (*model.SessionWithAvailability, error) {
	s := &model.SessionWithAvailability{}
	err := r.pool.QueryRow(ctx, `
		SELECT ps.id, ps.title, ps.description, ps.short_description, ps.category,
			ps.instructor_name, ps.tags, ps.start_at, ps.end_at, ps.seat_capacity,
			ps.price_minor_units, ps.currency, ps.registration_open_at, ps.registration_close_at,
			ps.requires_approval, ps.allows_waitlist, ps.status, ps.location, ps.created_by,
			ps.created_at, ps.updated_at,
			COALESCE(ssi.total_seats, ps.seat_capacity),
			COALESCE(ssi.reserved_seats, 0),
			COALESCE(ssi.available_seats, ps.seat_capacity)
		FROM program_sessions ps
		LEFT JOIN session_seat_inventory ssi ON ssi.session_id = ps.id
		WHERE ps.id = $1 AND ps.deleted_at IS NULL`, id,
	).Scan(
		&s.ID, &s.Title, &s.Description, &s.ShortDescription, &s.Category,
		&s.InstructorName, &s.Tags, &s.StartAt, &s.EndAt, &s.SeatCapacity,
		&s.PriceMinorUnits, &s.Currency, &s.RegistrationOpenAt, &s.RegistrationCloseAt,
		&s.RequiresApproval, &s.AllowsWaitlist, &s.Status, &s.Location, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt,
		&s.TotalSeats, &s.ReservedSeats, &s.AvailableSeats,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return s, nil
}

func (r *CatalogRepo) CreateSession(ctx context.Context, s *model.ProgramSession) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO program_sessions (id, title, description, short_description, category,
			instructor_name, tags, start_at, end_at, seat_capacity, price_minor_units, currency,
			registration_open_at, registration_close_at, requires_approval, allows_waitlist,
			status, location, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		s.ID, s.Title, s.Description, s.ShortDescription, s.Category,
		s.InstructorName, s.Tags, s.StartAt, s.EndAt, s.SeatCapacity,
		s.PriceMinorUnits, s.Currency, s.RegistrationOpenAt, s.RegistrationCloseAt,
		s.RequiresApproval, s.AllowsWaitlist, s.Status, s.Location, s.CreatedBy,
		s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	// Create seat inventory record
	_, err = tx.Exec(ctx, `
		INSERT INTO session_seat_inventory (session_id, total_seats, reserved_seats, updated_at)
		VALUES ($1, $2, 0, now())`, s.ID, s.SeatCapacity)
	if err != nil {
		return fmt.Errorf("insert seat inventory: %w", err)
	}

	return tx.Commit(ctx)
}

// --- Products ---

type ProductFilter struct {
	Query    string
	Status   string
	Category string
	Limit    int
	Offset   int
}

func (r *CatalogRepo) ListProducts(ctx context.Context, f ProductFilter) ([]model.ProductWithStock, int, error) {
	where := "WHERE p.deleted_at IS NULL"
	var args []interface{}
	argIdx := 1

	if f.Status != "" {
		where += fmt.Sprintf(" AND p.status = $%d", argIdx)
		args = append(args, f.Status)
		argIdx++
	}
	if f.Category != "" {
		where += fmt.Sprintf(" AND p.category = $%d", argIdx)
		args = append(args, f.Category)
		argIdx++
	}

	orderBy := "ORDER BY p.name ASC, p.id ASC"
	if f.Query != "" {
		where += fmt.Sprintf(` AND to_tsvector('english', coalesce(p.name,'') || ' ' || coalesce(p.short_description,'') || ' ' || coalesce(p.category,''))
			@@ plainto_tsquery('english', $%d)`, argIdx)
		args = append(args, f.Query)
		orderBy = fmt.Sprintf(`ORDER BY ts_rank(
			to_tsvector('english', coalesce(p.name,'') || ' ' || coalesce(p.short_description,'') || ' ' || coalesce(p.category,'')),
			plainto_tsquery('english', $%d)
		) DESC, p.name ASC, p.id ASC`, argIdx)
		argIdx++
	}

	baseQuery := fmt.Sprintf(`FROM products p
		LEFT JOIN product_inventory pi ON pi.product_id = p.id
		%s`, where)

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) "+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	selectQ := fmt.Sprintf(`SELECT p.id, p.name, p.description, p.short_description, p.category,
		p.sku, p.price_minor_units, p.currency, p.is_shippable, p.status, p.image_url, p.tags,
		p.created_by, p.created_at, p.updated_at,
		COALESCE(pi.stock_qty, 0)
		%s %s LIMIT $%d OFFSET $%d`, baseQuery, orderBy, argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var products []model.ProductWithStock
	for rows.Next() {
		var p model.ProductWithStock
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.ShortDescription, &p.Category,
			&p.SKU, &p.PriceMinorUnits, &p.Currency, &p.IsShippable, &p.Status,
			&p.ImageURL, &p.Tags, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
			&p.StockQty,
		); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

func (r *CatalogRepo) GetProductByID(ctx context.Context, id uuid.UUID) (*model.ProductWithStock, error) {
	p := &model.ProductWithStock{}
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.name, p.description, p.short_description, p.category,
			p.sku, p.price_minor_units, p.currency, p.is_shippable, p.status, p.image_url, p.tags,
			p.created_by, p.created_at, p.updated_at,
			COALESCE(pi.stock_qty, 0)
		FROM products p
		LEFT JOIN product_inventory pi ON pi.product_id = p.id
		WHERE p.id = $1 AND p.deleted_at IS NULL`, id,
	).Scan(
		&p.ID, &p.Name, &p.Description, &p.ShortDescription, &p.Category,
		&p.SKU, &p.PriceMinorUnits, &p.Currency, &p.IsShippable, &p.Status,
		&p.ImageURL, &p.Tags, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		&p.StockQty,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get product: %w", err)
	}
	return p, nil
}

func (r *CatalogRepo) CreateProduct(ctx context.Context, p *model.Product) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO products (id, name, description, short_description, category, sku,
			price_minor_units, currency, is_shippable, status, image_url, tags, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		p.ID, p.Name, p.Description, p.ShortDescription, p.Category, p.SKU,
		p.PriceMinorUnits, p.Currency, p.IsShippable, p.Status, p.ImageURL, p.Tags,
		p.CreatedBy, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "products_sku_key") {
			return fmt.Errorf("SKU already exists")
		}
		return fmt.Errorf("insert product: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO product_inventory (product_id, stock_qty, updated_at)
		VALUES ($1, 0, now())`, p.ID)
	if err != nil {
		return fmt.Errorf("insert product inventory: %w", err)
	}

	return tx.Commit(ctx)
}

// GetSessionsWithAvailableSeats returns session IDs that have available seats and waiting entries.
func (r *CatalogRepo) GetSessionsWithAvailableSeats(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT si.session_id FROM session_seat_inventory si
		JOIN session_waitlist_entries we ON we.session_id = si.session_id AND we.status = 'waiting'
		WHERE si.available_seats > 0
		GROUP BY si.session_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *CatalogRepo) GetSessionCategories(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT category FROM program_sessions
		WHERE deleted_at IS NULL AND category IS NOT NULL AND category != ''
		ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *CatalogRepo) GetProductCategories(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT category FROM products
		WHERE deleted_at IS NULL AND category IS NOT NULL AND category != ''
		ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}
