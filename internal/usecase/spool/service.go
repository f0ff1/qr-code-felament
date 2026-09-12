package spool

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type Service struct {
	repo spooldomain.Repository
}

func NewService(repo spooldomain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, material spooldomain.Material, color, manufacturer string, initialWeight int, price float64) (spooldomain.Spool, error) {
	if initialWeight <= 0 {
		return spooldomain.Spool{}, domain.ErrInvalid
	}

	entity := spooldomain.NewSpool(material, color, manufacturer, initialWeight, price)
	if err := s.repo.Create(ctx, entity); err != nil {
		return spooldomain.Spool{}, err
	}
	return entity, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (spooldomain.Spool, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return spooldomain.Spool{}, err
	}
	return entity, nil
}

func (s *Service) GetByQRToken(ctx context.Context, token string) (spooldomain.Spool, error) {
	return s.repo.GetByQRToken(ctx, token)
}

func (s *Service) GetPublicView(ctx context.Context, token string) (PublicSpoolView, error) {
	spoolEntity, err := s.repo.GetByQRToken(ctx, token)
	if err != nil {
		return PublicSpoolView{}, err
	}
	return PublicSpoolView{
		Material:        string(spoolEntity.Material),
		Color:           spoolEntity.Color,
		RemainingWeight: spoolEntity.CurrentWeight,
		InitialWeight:   spoolEntity.InitialWeight,
		Status:          string(spoolEntity.Status),
		QRToken:         spoolEntity.QRToken,
	}, nil
}

func (s *Service) List(ctx context.Context) ([]spooldomain.Spool, error) {
	return s.repo.List(ctx)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Consume(ctx context.Context, id uuid.UUID, weight int) error {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if weight <= 0 {
		return fmt.Errorf("%w: weight must be positive", domain.ErrInvalid)
	}
	if entity.CurrentWeight < weight {
		return fmt.Errorf("%w: not enough material on spool", domain.ErrInvalid)
	}
	entity.Consume(weight)
	return s.repo.Update(ctx, entity)
}
