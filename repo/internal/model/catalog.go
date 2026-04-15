package model

import (
	"time"

	"github.com/google/uuid"
)

// ProgramSession represents a scheduled session offering.
type ProgramSession struct {
	ID                   uuid.UUID  `json:"id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	ShortDescription     string     `json:"short_description"`
	Category             *string    `json:"category,omitempty"`
	InstructorName       *string    `json:"instructor_name,omitempty"`
	Tags                 []string   `json:"tags"`
	StartAt              time.Time  `json:"start_at"`
	EndAt                time.Time  `json:"end_at"`
	SeatCapacity         int        `json:"seat_capacity"`
	PriceMinorUnits      int64      `json:"price_minor_units"`
	Currency             string     `json:"currency"`
	RegistrationOpenAt   *time.Time `json:"registration_open_at,omitempty"`
	RegistrationCloseAt  *time.Time `json:"registration_close_at,omitempty"`
	RequiresApproval     bool       `json:"requires_approval"`
	AllowsWaitlist       bool       `json:"allows_waitlist"`
	Status               string     `json:"status"`
	Location             *string    `json:"location,omitempty"`
	CreatedBy            *uuid.UUID `json:"created_by,omitempty"`
	DeletedAt            *time.Time `json:"-"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

const (
	SessionStatusDraft     = "draft"
	SessionStatusPublished = "published"
	SessionStatusCanceled  = "canceled"
	SessionStatusArchived  = "archived"
)

// SessionWithAvailability includes live seat availability.
type SessionWithAvailability struct {
	ProgramSession
	TotalSeats     int `json:"total_seats"`
	ReservedSeats  int `json:"reserved_seats"`
	AvailableSeats int `json:"available_seats"`
}

// SeatInventory tracks authoritative seat counts.
type SeatInventory struct {
	SessionID      uuid.UUID `json:"session_id"`
	TotalSeats     int       `json:"total_seats"`
	ReservedSeats  int       `json:"reserved_seats"`
	AvailableSeats int       `json:"available_seats"`
	Version        int       `json:"version"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Product represents a purchasable merchandise item.
type Product struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	ShortDescription string     `json:"short_description"`
	Category         *string    `json:"category,omitempty"`
	SKU              *string    `json:"sku,omitempty"`
	PriceMinorUnits  int64      `json:"price_minor_units"`
	Currency         string     `json:"currency"`
	IsShippable      bool       `json:"is_shippable"`
	Status           string     `json:"status"`
	ImageURL         *string    `json:"image_url,omitempty"`
	Tags             []string   `json:"tags"`
	CreatedBy        *uuid.UUID `json:"created_by,omitempty"`
	DeletedAt        *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

const (
	ProductStatusDraft        = "draft"
	ProductStatusPublished    = "published"
	ProductStatusDiscontinued = "discontinued"
	ProductStatusArchived     = "archived"
)

// ProductWithStock includes live stock availability.
type ProductWithStock struct {
	Product
	StockQty int `json:"stock_qty"`
}

// ProductInventory tracks authoritative stock counts.
type ProductInventory struct {
	ProductID uuid.UUID `json:"product_id"`
	StockQty  int       `json:"stock_qty"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}
