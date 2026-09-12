# Architecture

## Dependency direction

```text
Delivery
  ↓
UseCase
  ↓
Domain
  ↑
Repository / Infrastructure
```

This keeps the business rules free from web, database and third-party concerns.

## Domain modules

- Spool
- Printer
- PrintJob
- Product
- Inventory
- FilamentTransaction

## Operational modules

- HTTP delivery
- SSE delivery
- PostgreSQL repositories
- Redis cache
- Printer adapters
- Worker scheduler
- Notifications

## Phased roadmap

1. bootstrap project
2. spool CRUD and QR token generation
3. inventory ledger and weight tracking
4. printer abstraction and mock adapter
5. print job orchestration
6. SSE and real-time updates
7. prediction engine and notifications
8. statistics and production business logic
9. production hardening

This structure supports a clean evolution from a modular monolith to service boundaries later.
