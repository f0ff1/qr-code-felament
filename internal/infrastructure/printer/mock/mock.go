package mock

import (
	"context"
	"math/rand"

	"filamenttracker/internal/domain/printer"
)

type Adapter struct{}

func (Adapter) GetStatus(ctx context.Context, p printer.Printer) (printer.PrinterStatus, error) {
	if rand.Intn(10) == 0 {
		return printer.StatusOffline, nil
	}
	if rand.Intn(2) == 0 {
		return printer.StatusPrinting, nil
	}
	return printer.StatusIdle, nil
}

func (Adapter) GetProgress(ctx context.Context, p printer.Printer) (float64, error) {
	return float64(rand.Intn(100)), nil
}

func (Adapter) Sync(ctx context.Context, p printer.Printer) error {
	return nil
}
