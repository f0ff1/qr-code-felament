package printer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"filamenttracker/internal/domain"
	printerdomain "filamenttracker/internal/domain/printer"
	"filamenttracker/internal/infrastructure/printer/bambu"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, p printerdomain.Printer) error
	GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
	List(ctx context.Context) ([]printerdomain.Printer, error)
	Update(ctx context.Context, p printerdomain.Printer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CreateInput struct {
	Name            string
	Model           string
	LANHost         string
	LANSerial       string
	LANAccessCode   string
	LANEnabled      bool
	CloudEnabled    bool
	CloudEmail      string
	CloudPassword   string
	CloudToken      string
	CloudRegion     string
	CloudVerifyCode string
	DefaultSpoolID  uuid.UUID
}

type Service struct {
	repo    Repository
	adapter printerdomain.Adapter
}

func NewService(repo Repository, adapter printerdomain.Adapter) *Service {
	return &Service{repo: repo, adapter: adapter}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (printerdomain.Printer, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Model) == "" {
		return printerdomain.Printer{}, fmt.Errorf("%w: printer name and model are required", domain.ErrInvalid)
	}
	if input.LANEnabled && input.CloudEnabled {
		return printerdomain.Printer{}, fmt.Errorf("%w: choose either LAN or Cloud connection", domain.ErrInvalid)
	}
	if err := validateConnection(input); err != nil {
		return printerdomain.Printer{}, err
	}

	p := printerdomain.NewPrinter(strings.TrimSpace(input.Name), strings.TrimSpace(input.Model))
	applyConnection(&p, input)

	if p.CloudEnabled {
		if _, err := authenticateCloud(&p, input.CloudVerifyCode); err != nil {
			return printerdomain.Printer{}, err
		}
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) UpdateLAN(ctx context.Context, id uuid.UUID, input CreateInput) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	if input.LANEnabled && input.CloudEnabled {
		return printerdomain.Printer{}, fmt.Errorf("%w: choose either LAN or Cloud connection", domain.ErrInvalid)
	}
	if strings.TrimSpace(input.Name) != "" {
		p.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Model) != "" {
		p.Model = strings.TrimSpace(input.Model)
	}
	if input.CloudEnabled && strings.TrimSpace(input.CloudPassword) == "" {
		input.CloudPassword = p.CloudPassword
	}
	if input.CloudEnabled && strings.TrimSpace(input.CloudToken) == "" {
		input.CloudToken = p.CloudToken
	}
	if input.LANEnabled && strings.TrimSpace(input.LANAccessCode) == "" {
		input.LANAccessCode = p.LANAccessCode
	}
	if err := validateConnection(input); err != nil {
		return printerdomain.Printer{}, err
	}
	applyConnection(&p, input)
	p.UpdatedAt = time.Now()

	if p.CloudEnabled && !p.CloudLinked() {
		if _, err := authenticateCloud(&p, input.CloudVerifyCode); err != nil {
			return printerdomain.Printer{}, err
		}
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) VerifyCloud(ctx context.Context, id uuid.UUID, code string) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	if !p.CloudEnabled {
		return printerdomain.Printer{}, fmt.Errorf("%w: cloud is not enabled for this printer", domain.ErrInvalid)
	}
	token, err := bambu.CloudVerify(p.CloudEmail, code, p.CloudRegion)
	if err != nil {
		return printerdomain.Printer{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	p.CloudToken = token
	p.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]printerdomain.Printer, error) {
	return s.repo.List(ctx)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) SyncStatus(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	status, err := s.adapter.GetStatus(ctx, p)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	p.Status = status
	p.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) GetProgress(ctx context.Context, id uuid.UUID) (float64, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	return s.adapter.GetProgress(ctx, p)
}

func validateConnection(input CreateInput) error {
	if input.LANEnabled {
		if strings.TrimSpace(input.LANHost) == "" || strings.TrimSpace(input.LANSerial) == "" || strings.TrimSpace(input.LANAccessCode) == "" {
			return fmt.Errorf("%w: LAN host, serial and access code are required when LAN is enabled", domain.ErrInvalid)
		}
	}
	if input.CloudEnabled {
		if strings.TrimSpace(input.LANSerial) == "" {
			return fmt.Errorf("%w: printer serial is required for Cloud", domain.ErrInvalid)
		}
		hasToken := strings.TrimSpace(input.CloudToken) != ""
		hasCreds := strings.TrimSpace(input.CloudEmail) != "" && strings.TrimSpace(input.CloudPassword) != ""
		if !hasToken && !hasCreds {
			return fmt.Errorf("%w: cloud email/password or access token is required", domain.ErrInvalid)
		}
	}
	return nil
}

func applyConnection(p *printerdomain.Printer, input CreateInput) {
	p.LANHost = strings.TrimSpace(input.LANHost)
	p.LANSerial = strings.TrimSpace(input.LANSerial)
	p.LANAccessCode = strings.TrimSpace(input.LANAccessCode)
	p.LANEnabled = input.LANEnabled && !input.CloudEnabled
	p.CloudEnabled = input.CloudEnabled
	p.CloudEmail = strings.TrimSpace(input.CloudEmail)
	p.CloudPassword = strings.TrimSpace(input.CloudPassword)
	p.CloudToken = strings.TrimSpace(input.CloudToken)
	p.CloudRegion = bambu.NormalizeCloudRegion(input.CloudRegion)
	p.DefaultSpoolID = input.DefaultSpoolID
	if !p.CloudEnabled {
		p.CloudEmail = ""
		p.CloudPassword = ""
		p.CloudToken = ""
		p.CloudRegion = "us"
	}
	if !p.LANEnabled {
		p.LANHost = ""
		p.LANAccessCode = ""
		if !p.CloudEnabled {
			p.LANSerial = ""
		}
	}
}

func authenticateCloud(p *printerdomain.Printer, verifyCode string) (bambu.CloudLoginResult, error) {
	if strings.TrimSpace(p.CloudToken) != "" {
		return bambu.CloudLoginResult{Token: p.CloudToken}, nil
	}
	if strings.TrimSpace(verifyCode) != "" {
		token, err := bambu.CloudVerify(p.CloudEmail, verifyCode, p.CloudRegion)
		if err != nil {
			return bambu.CloudLoginResult{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
		}
		p.CloudToken = token
		return bambu.CloudLoginResult{Token: token}, nil
	}
	result, err := bambu.CloudLogin(p.CloudEmail, p.CloudPassword, p.CloudRegion)
	if err != nil {
		return bambu.CloudLoginResult{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	if result.NeedsVerify {
		return result, nil
	}
	p.CloudToken = result.Token
	return result, nil
}
