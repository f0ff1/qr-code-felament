# Filament Tracker

Filament Tracker is a modular Go monolith for tracking filament, printers, print jobs, and manufacturing costs.

## Goals

- Track spools and their remaining weight
- Generate QR codes for spool lookup
- Monitor printers and print jobs
- Forecast filament exhaustion during active prints
- Maintain a ledger of material transactions
- Support future extraction to separate services without rewriting the domain

## Project structure

- `cmd/api` — HTTP API entry point
- `cmd/worker` — background workers
- `internal/domain` — business entities and interfaces
- `internal/usecase` — application logic
- `internal/delivery` — HTTP and event delivery adapters
- `internal/repository` — persistence implementations
- `internal/infrastructure` — QR, printers, notifications, clocks
- `internal/worker` — scheduler and worker orchestration

## Getting started

```bash
go mod download
go test ./...
```

## Architecture principles

- Domain has no knowledge of HTTP, DB or frameworks.
- Use cases depend on domain interfaces and not on concrete storage.
- Infrastructure implements interfaces from the domain.
- The architecture is designed for later extraction into independent services if needed.
