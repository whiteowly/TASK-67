package service

import (
	"context"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

type CatalogService struct {
	catalogRepo *repo.CatalogRepo
}

func NewCatalogService(catalogRepo *repo.CatalogRepo) *CatalogService {
	return &CatalogService{catalogRepo: catalogRepo}
}

func (s *CatalogService) ListSessions(ctx context.Context, f repo.SessionFilter) ([]model.SessionWithAvailability, int, error) {
	return s.catalogRepo.ListSessions(ctx, f)
}

func (s *CatalogService) GetSession(ctx context.Context, id uuid.UUID) (*model.SessionWithAvailability, error) {
	return s.catalogRepo.GetSessionByID(ctx, id)
}

func (s *CatalogService) ListProducts(ctx context.Context, f repo.ProductFilter) ([]model.ProductWithStock, int, error) {
	return s.catalogRepo.ListProducts(ctx, f)
}

func (s *CatalogService) GetProduct(ctx context.Context, id uuid.UUID) (*model.ProductWithStock, error) {
	return s.catalogRepo.GetProductByID(ctx, id)
}

func (s *CatalogService) GetSessionCategories(ctx context.Context) ([]string, error) {
	return s.catalogRepo.GetSessionCategories(ctx)
}

func (s *CatalogService) GetProductCategories(ctx context.Context) ([]string, error) {
	return s.catalogRepo.GetProductCategories(ctx)
}
