package printer

import "context"

type Adapter interface {
	GetStatus(ctx context.Context, p Printer) (PrinterStatus, error)
	GetProgress(ctx context.Context, p Printer) (float64, error)
	Sync(ctx context.Context, p Printer) error
}
