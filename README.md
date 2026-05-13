<div align="center">

# Plants Accounting

**Internal accounting tool for a plant-nursery business**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen?style=flat-square)](go.mod)
[![Tests](https://img.shields.io/badge/tests-109%20passing-brightgreen?style=flat-square)]()
[![Coverage](https://img.shields.io/badge/coverage-84%25-brightgreen?style=flat-square)]()
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

</div>

---

A **production tool** built for a real plant-nursery and landscaping business that serves cafés and restaurants. Replaces a stack of Excel sheets with a single zero-dependency Go binary: retail sales across multiple channels (market, Instagram, Telegram, Kufar), landscaping projects, automatic stock deduction, expense tracking, and profit-share calculation for the co-owners.

Single-user by design: runs locally or on a small internal server, no authentication, no SaaS, no `npm install`. The UI is plain HTML/CSS/JS — no build step, no Node toolchain.

---

## Features

### Accounting
- **Inventory** — plant catalog with category, size, price and quantity. Inline editing for every field. Atomic stock deduction on sales and projects.
- **Retail sales** — recorded per sales channel with filtering and search. `skip_stock` mode for services and off-catalog items that should not touch the inventory.
- **Landscaping projects** — orders for cafés and restaurants: client, list of plants used, labor cost. Plants are deducted from stock on creation and returned on delete.
- **Expenses** — eight categories (purchasing, rent, transport, advertising, ...) with an inline entry form.
- **CSV import** — load a price list from Google Sheets via a 3-step flow that diffs against the current catalog.

### Analytics
- **Dashboard** — four KPI cards (revenue, expenses, profit, orders) with delta to the previous period. 12-month sales area chart. Channel donut. Top-5 plants.
- **Salaries** — automatic split of net profit by configurable percentages, validated to sum to 100%.
- **Backups** — list of rotating snapshots with metadata, available in settings.

### UX
- Design system on CSS custom properties (Inter, accent green, dark sidebar)
- Eight pages, slide-in panels for creating sales and projects
- Toast notifications, confirm modals for destructive actions
- Row fade-in with stagger, flash highlight on freshly created rows
- Keyboard-friendly: Enter submits forms, Esc closes panels

---

## Quick start

```bash
git clone https://github.com/cacarerru-prog/plan-accounting.git
cd plan-accounting
go run ./cmd/server
```

Open [http://localhost:3000](http://localhost:3000).

Custom port:
```bash
PORT=8080 go run ./cmd/server
```

Run the tests:
```bash
go test ./... -cover
```

Build a static binary:
```bash
go build -o plants ./cmd/server
./plants
```

**Requirements:** Go 1.21+. No database, no message broker, no container runtime.

---

## Architecture

```
cmd/
└── server/main.go        — entry point, graceful shutdown, env-based config
internal/
├── models/               — domain types (Plant, Sale, Expense, Project, Employee)
├── storage/              — Store with sync.RWMutex, JSON file, backups, aggregates
│   ├── storage.go        — Load / SaveNow / atomic write / rotateBackup
│   ├── plants.go         — plant CRUD with duplicate-name protection
│   ├── sales.go          — sales with stock deduction, ErrInsufficientQty
│   ├── projects.go       — projects with atomic multi-plant deduction
│   ├── expenses.go       — expenses
│   ├── employees.go      — co-owners (profit-share recipients)
│   ├── stats.go          — period aggregates + delta to previous period + 12-month trend
│   ├── backups.go        — list of snapshot files
│   ├── import_csv.go     — Google Sheets price-list import
│   └── *_test.go         — 109 unit tests, ~84% coverage
└── api/                  — HTTP handlers (one file per resource)
static/
└── index.html            — single-file SPA (vanilla HTML/CSS/JS, zero deps)
```

### Engineering decisions

- **Atomic writes** — `data.json.tmp` + `os.Rename` guarantee the file cannot end up half-written if the process crashes mid-save.
- **Rotating backups** — once an hour a snapshot is written to `backups/`, the last 30 are kept and older ones are pruned automatically.
- **`log/slog`** — structured logging from the Go 1.21+ standard library, no third-party logger.
- **`sync.RWMutex`** — race-free concurrent access. Reads run under `RLock`, writes under `Lock`. `Snapshot()` copies the data under `RLock` so the stats computation never blocks writers.
- **Graceful shutdown** — the server waits for in-flight requests and flushes state on `SIGTERM` / `SIGINT`.
- **Monotone interpolation** (Fritsch–Carlson) on the trend chart so the curve never dips below zero with sparse data, unlike naive cubic-bezier smoothing.
- **Zero dependencies** — only the Go standard library. There is no `go.sum`.

---

## API

All endpoints return JSON and accept JSON bodies on `POST` / `PUT`.

### Resources

| Method   | Path                  | Description                                              |
|----------|-----------------------|----------------------------------------------------------|
| `GET`    | `/api/plants`         | List plants                                              |
| `POST`   | `/api/plants`         | Create a plant (duplicate-name protected)                |
| `PUT`    | `/api/plants/:id`     | Update a plant                                           |
| `DELETE` | `/api/plants/:id`     | Delete a plant                                           |
| `GET`    | `/api/sales`          | Sales history (up to 2000 latest)                        |
| `POST`   | `/api/sales`          | Create a sale (atomic stock deduction)                   |
| `DELETE` | `/api/sales/:id`      | Delete a sale (returns the quantity to stock)            |
| `GET`    | `/api/projects`       | List projects                                            |
| `POST`   | `/api/projects`       | Create a project (atomic deduction of all line items)    |
| `DELETE` | `/api/projects/:id`   | Delete a project (returns all line items to stock)       |
| `GET`    | `/api/expenses`       | List expenses                                            |
| `POST`   | `/api/expenses`       | Create an expense                                        |
| `DELETE` | `/api/expenses/:id`   | Delete an expense                                        |
| `GET`    | `/api/employees`      | List co-owners and their profit shares                   |
| `POST`   | `/api/employees`      | Create a co-owner                                        |
| `PUT`    | `/api/employees/:id`  | Update share or name                                     |
| `DELETE` | `/api/employees/:id`  | Delete a co-owner                                        |

### Analytics and maintenance

| Method   | Path                          | Description                                                |
|----------|-------------------------------|------------------------------------------------------------|
| `GET`    | `/api/stats?month=M&year=Y`   | KPIs for the period + delta vs. previous month + salaries  |
| `GET`    | `/api/stats/monthly?year=Y`   | 12 monthly points (revenue / expenses / profit)            |
| `GET`    | `/api/backups`                | List of snapshot files with metadata                       |
| `POST`   | `/api/import/csv`             | Import a price list from CSV (multipart upload)            |

---

## Stack

| Layer    | Technology                                            |
|----------|-------------------------------------------------------|
| Backend  | Go 1.21 · `net/http` · `encoding/json` · `log/slog`   |
| Frontend | Vanilla HTML / CSS / JavaScript · hand-rolled SVG charts |
| Storage  | JSON file with atomic writes + rotating backups       |
| Deps     | 0 (standard library only)                             |

---

## Testing

Coverage by package after `go test ./... -cover`:

| Package              | Coverage |
|----------------------|----------|
| `internal/storage`   | 88.3%    |
| `internal/api`       | 88.0%    |
| `internal/models`    | 100%     |
| **Total**            | **~84%** |

Storage tests cover CRUD, atomic project deduction, stock restoration on delete,
CSV import (Russian decimal comma, BOM, header detection, totals/subtotals skipping),
period filtering, monthly trend, and backup rotation.
API tests cover happy paths plus validation, error mapping (404 / 409 / 405),
malformed JSON, and the multipart CSV upload flow.

---

## Screenshots

> Screenshots will be added under `docs/screenshots/` in a future update.

---

## Configuration

| Variable | Default  | Purpose          |
|----------|----------|------------------|
| `PORT`   | `3000`   | HTTP server port |

The data file is `data.json` next to the binary. Backups are written to `backups/`.

---

## License

MIT — see [LICENSE](LICENSE).

---

## Author

**Aliaksandr Kacheuski** — telecom-engineering student at BSUIR (Minsk), learning Go.
Built for a real family business and used daily.

[github.com/cacarerru-prog](https://github.com/cacarerru-prog)
