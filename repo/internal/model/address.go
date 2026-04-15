package model

import (
	"time"

	"github.com/google/uuid"
)

type DeliveryAddress struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	Label         string     `json:"label"`
	RecipientName string     `json:"recipient_name"`
	Phone         string     `json:"phone"`
	Line1         string     `json:"line1"`
	Line2         string     `json:"line2"`
	City          string     `json:"city"`
	State         string     `json:"state"`
	PostalCode    string     `json:"postal_code"`
	CountryCode   string     `json:"country_code"`
	IsDefault     bool       `json:"is_default"`
	DeletedAt     *time.Time `json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
