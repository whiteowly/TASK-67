package service

import (
	"context"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

type AddressService struct {
	addressRepo *repo.AddressRepo
}

func NewAddressService(addressRepo *repo.AddressRepo) *AddressService {
	return &AddressService{addressRepo: addressRepo}
}

func (s *AddressService) List(ctx context.Context, userID uuid.UUID) ([]model.DeliveryAddress, error) {
	return s.addressRepo.ListByUser(ctx, userID)
}

func (s *AddressService) Get(ctx context.Context, id, userID uuid.UUID) (*model.DeliveryAddress, error) {
	addr, err := s.addressRepo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if addr == nil {
		return nil, fmt.Errorf("address not found")
	}
	return addr, nil
}

type AddressInput struct {
	Label         string
	RecipientName string
	Phone         string
	Line1         string
	Line2         string
	City          string
	State         string
	PostalCode    string
	CountryCode   string
	IsDefault     bool
}

func (i *AddressInput) Validate() []string {
	var errs []string
	if i.RecipientName == "" {
		errs = append(errs, "recipient_name is required")
	}
	if i.Phone == "" {
		errs = append(errs, "phone is required")
	}
	if i.Line1 == "" {
		errs = append(errs, "line1 is required")
	}
	if i.City == "" {
		errs = append(errs, "city is required")
	}
	return errs
}

func (s *AddressService) Create(ctx context.Context, userID uuid.UUID, input AddressInput) (*model.DeliveryAddress, error) {
	if errs := input.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("validation: %s", errs[0])
	}

	now := time.Now().UTC()
	cc := input.CountryCode
	if cc == "" {
		cc = "CN"
	}

	addr := &model.DeliveryAddress{
		ID:            uuid.New(),
		UserID:        userID,
		Label:         input.Label,
		RecipientName: input.RecipientName,
		Phone:         input.Phone,
		Line1:         input.Line1,
		Line2:         input.Line2,
		City:          input.City,
		State:         input.State,
		PostalCode:    input.PostalCode,
		CountryCode:   cc,
		IsDefault:     input.IsDefault,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.addressRepo.Create(ctx, addr); err != nil {
		return nil, err
	}
	return addr, nil
}

func (s *AddressService) Update(ctx context.Context, id, userID uuid.UUID, input AddressInput) (*model.DeliveryAddress, error) {
	if errs := input.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("validation: %s", errs[0])
	}

	addr, err := s.addressRepo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if addr == nil {
		return nil, fmt.Errorf("address not found")
	}

	addr.Label = input.Label
	addr.RecipientName = input.RecipientName
	addr.Phone = input.Phone
	addr.Line1 = input.Line1
	addr.Line2 = input.Line2
	addr.City = input.City
	addr.State = input.State
	addr.PostalCode = input.PostalCode
	if input.CountryCode != "" {
		addr.CountryCode = input.CountryCode
	}
	addr.IsDefault = input.IsDefault

	if err := s.addressRepo.Update(ctx, addr); err != nil {
		return nil, err
	}
	return addr, nil
}

func (s *AddressService) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.addressRepo.SoftDelete(ctx, id, userID)
}
