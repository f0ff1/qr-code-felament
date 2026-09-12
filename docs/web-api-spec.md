# WEB API Specification for Filament Tracker

This document describes the HTTP API and data contracts used by the browser UI for the Filament Tracker application.
It is intended as a reference for building the next web interface (HTML/CSS/JS) without reading the Go implementation.

## Base URL

- Local app: http://localhost:8080
- All endpoints below are relative to that base URL.

## Common conventions

- JSON is used for request and response bodies.
- Successful responses usually return HTTP 200 or 201.
- Errors are returned as plain text with a standard HTTP status code.
- IDs are UUID strings.
- Time values are ISO 8601 / RFC3339 strings.

---

## 1. Health

### GET /api/health

Checks service availability.

Response:

```json
{"status":"ok"}
```

Status: 200 OK

---

## 2. Spools

### GET /api/spools

Returns all spools.

Response example:

```json
[
  {
    "id": "8d8911d4-7c0d-4f28-b1cb-2ff78d7d2444",
    "material": "PLA",
    "color": "Black",
    "manufacturer": "eSUN",
    "initial_weight": 1000,
    "remaining_weight": 640,
    "status": "available",
    "qr_token": "8a7f3d2c"
  }
]
```

Status: 200 OK

### POST /api/spools

Creates a new spool.

Request body:

```json
{
  "material": "PLA",
  "color": "Black",
  "manufacturer": "eSUN",
  "initialWeight": 1000,
  "price": 30
}
```

Response example:

```json
{
  "id": "8d8911d4-7c0d-4f28-b1cb-2ff78d7d2444",
  "qr_token": "8a7f3d2c"
}
```

Status: 201 Created

### Public spool page

### GET /public/spools/{qr_token}

Returns public information for a spool by QR token.

Response example:

```json
{
  "material": "PLA",
  "color": "Black",
  "remaining_weight": 640,
  "initial_weight": 1000,
  "status": "available",
  "qr_token": "8a7f3d2c"
}
```

Status: 200 OK

If not found: 404

---

## 3. Printers

### GET /api/printers

Returns all printers.

Response example:

```json
[
  {
    "id": "0f9b1cad-2b7b-4478-a224-e249bc4d78d9",
    "name": "Bambu A1",
    "model": "Bambu Lab",
    "status": "idle"
  }
]
```

Status: 200 OK

### POST /api/printers

Creates a new printer.

Request body:

```json
{
  "name": "Bambu A1",
  "model": "Bambu Lab"
}
```

Response example:

```json
{
  "id": "0f9b1cad-2b7b-4478-a224-e249bc4d78d9",
  "status": "idle"
}
```

Status: 201 Created

---

## 4. Products

### GET /api/products

Returns all products.

Response example:

```json
[
  {
    "id": "d4ce38f7-83cd-4d38-80ef-9d7f88315859",
    "name": "Dragon",
    "description": "Created from dashboard",
    "material": "PLA",
    "estimated_weight": 200,
    "estimated_print_time": "4h30m0s",
    "price": 0
  }
]
```

Status: 200 OK

### POST /api/products

Creates a new product.

Request body:

```json
{
  "name": "Dragon",
  "description": "Created from dashboard",
  "material": "PLA",
  "estimatedWeight": 200,
  "estimatedPrintTime": "4h30m",
  "price": 0
}
```

Notes:
- estimatedPrintTime is interpreted as Go duration string, for example:
  - "4h30m"
  - "2h15m"
  - "45m"
  - "90m"

Response example:

```json
{
  "id": "d4ce38f7-83cd-4d38-80ef-9d7f88315859",
  "name": "Dragon"
}
```

Status: 201 Created

---

## 5. Print jobs

### GET /api/print-jobs

Returns all jobs.

Response example:

```json
[
  {
    "id": "79b23998-989d-4d1c-b95a-81579e978938",
    "printer_id": "0f9b1cad-2b7b-4478-a224-e249bc4d78d9",
    "product_id": "d4ce38f7-83cd-4d38-80ef-9d7f88315859",
    "spool_id": "8d8911d4-7c0d-4f28-b1cb-2ff78d7d2444",
    "status": "printing",
    "progress": 42
  }
]
```

Status: 200 OK

### POST /api/print-jobs

Starts a new print job.

Request body:

```json
{
  "printerId": "0f9b1cad-2b7b-4478-a224-e249bc4d78d9",
  "productId": "d4ce38f7-83cd-4d38-80ef-9d7f88315859",
  "spoolId": "8d8911d4-7c0d-4f28-b1cb-2ff78d7d2444"
}
```

Response example:

```json
{
  "id": "79b23998-989d-4d1c-b95a-81579e978938",
  "status": "printing"
}
```

Status: 201 Created

---

## 6. Inventory

### GET /api/inventory/summary

Returns aggregate material summary by material/color.

Response example:

```json
{
  "items": [
    {
      "material": "PLA",
      "color": "Black",
      "total": 640
    },
    {
      "material": "PLA",
      "color": "White",
      "total": 210
    }
  ]
}
```

Status: 200 OK

### GET /api/inventory/transactions

Returns ledger entries for spool consumption / material movements.

Response example:

```json
[
  {
    "id": "2026-09-12T09:15:22.991Z",
    "spool_id": "8d8911d4-7c0d-4f28-b1cb-2ff78d7d2444",
    "type": "CONSUMPTION",
    "weight": -180,
    "created_at": "2026-09-12T09:15:22Z"
  }
]
```

Status: 200 OK

### POST /api/inventory/consume

Consumes material from a spool and records a transaction.

Request body:

```json
{
  "spoolId": "8d8911d4-7c0d-4f28-b1cb-2ff78d7d2444",
  "weight": 180
}
```

Response example:

```json
{
  "status": "consumed"
}
```

Status: 201 Created

---

## 7. Forecast / filament estimation

### GET /api/forecast

Calculates whether the currently available filament is sufficient for a desired print requirement.

Query params:

- requiredWeight: integer, required total filament in grams

Example:

```text
GET /api/forecast?requiredWeight=200
```

Response example:

```json
{
  "available": 940,
  "requiredWeight": 200,
  "missing": 0,
  "canFinish": true,
  "estimatedRemaining": "3h10m0s",
  "explanation": "Enough filament: 940g available, 200g required"
}
```

Status: 200 OK

---

## 8. Event stream SSE

### GET /events

Server-Sent Events endpoint for live status updates.

The stream emits event blocks like:

```text
event: spool_low
data: {"id":"2026-09-12T09:15:22.991Z","type":"spool_low","message":"Spool ... is running low","created_at":"2026-09-12T09:15:22Z"}

```

Example event payload:

```json
{
  "id": "2026-09-12T09:15:22.991Z",
  "type": "print_started",
  "message": "Print job started",
  "created_at": "2026-09-12T09:15:22Z"
}
```

The client can listen with EventSource and handle onmessage / event parsing.

---

## 9. Core entity models

### Spool

```json
{
  "id": "uuid",
  "qr_token": "string",
  "material": "PLA | PETG | ABS",
  "color": "string",
  "manufacturer": "string",
  "initial_weight": 1000,
  "remaining_weight": 640,
  "status": "available | in_use | empty",
  "price": 30.0,
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### Printer

```json
{
  "id": "uuid",
  "name": "string",
  "model": "string",
  "status": "idle | printing | paused | offline",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### Product

```json
{
  "id": "uuid",
  "name": "string",
  "description": "string",
  "material": "string",
  "estimated_weight": 200,
  "estimated_print_time": "duration string",
  "price": 0.0
}
```

### Print job

```json
{
  "id": "uuid",
  "printer_id": "uuid",
  "product_id": "uuid",
  "spool_id": "uuid",
  "status": "queued | printing | paused | completed | failed | cancelled",
  "progress": 42,
  "estimated_weight": 200,
  "consumed_weight": 0,
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### Inventory transaction

```json
{
  "id": "string",
  "spool_id": "uuid",
  "type": "CONSUMPTION",
  "weight": -180,
  "created_at": "timestamp"
}
```

---

## 10. Typical frontend flow

1. Load dashboard data:
   - GET /api/spools
   - GET /api/printers
   - GET /api/products
   - GET /api/print-jobs
2. Load forecast info:
   - GET /api/forecast?requiredWeight=...
3. Open live updates:
   - EventSource /events
4. Create records:
   - POST /api/spools
   - POST /api/printers
   - POST /api/products
   - POST /api/print-jobs
5. Handle material consumption:
   - POST /api/inventory/consume

---

## 11. UI requirements implied by the API

The web UI should support:

- spool inventory list with remaining weight and status
- printer status cards
- product list and print-time values
- print job creation form
- live event log
- low-stock warning handling
- public spool lookup by QR token
- material consumption / summary dashboard

---

## 12. Important notes for implementation

- Some fields are camelCase in requests, but API responses use snake_case.
- Example: request uses `initialWeight`, response uses `initial_weight`.
- Product print time is passed as a Go-style duration string, not a raw integer.
- The event stream is SSE, not WebSocket.
- The app uses in-memory repositories; persistence is not yet implemented.

This spec is enough for building the HTML pages and their interaction logic for the web interface.
