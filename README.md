<div align="center">

# Plant Accounting

**Инструмент учёта продаж для бизнеса на озеленении**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen?style=flat-square)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

</div>

---

Это не учебный проект — это инструмент, которым пользуются каждый день.

Компания продаёт растения на рынке, через Instagram, Telegram и Куфар, а также озеленяет заведения общественного питания. Excel не справлялся: продажи из разных каналов, склад, расходы, зарплаты по проценту от прибыли — всё в одном месте. Plant Accounting заменил таблицы одним бинарником.

---

## Возможности

**Склад** — каталог растений с наличием. При продаже количество списывается автоматически. Поддерживает режим `skip_stock` для услуг и товаров не из каталога.

**Продажи** — учёт по каналам (рынок, Instagram, Telegram, Куфар, прочее). Дата, количество, цена, итог.

**Проекты** — для заказов на озеленение заведений. К проекту привязан клиент, список растений со стоимостью и стоимость работ. При создании проекта склад списывается автоматически.

**Расходы** — по категориям: закупка, аренда, транспорт, прочее.

**Зарплаты** — автоматический расчёт для каждого сотрудника по проценту от чистой прибыли.

**Дашборд** — выручка, расходы, прибыль, топ-5 растений по продажам, разбивка по каналам.

**Импорт CSV** — загрузка прайс-листа из Google Sheets одним запросом.

---

## Быстрый старт

```bash
git clone https://github.com/cacarerru-prog/plant-accounting.git
cd plant-accounting
go run ./cmd/server
```

Открой браузер: [http://localhost:8080](http://localhost:8080)

Требования: Go 1.21+. Никаких баз данных, брокеров, контейнеров и `npm install`.

Прогнать тесты:
```bash
go test ./... -cover
```

---

## Архитектура

```
cmd/
└── server/main.go        — точка входа, graceful shutdown
internal/
├── models/               — доменные структуры (Plant, Sale, Expense, ...)
├── storage/              — Store с sync.RWMutex, JSON-файл, бэкапы, CSV-импорт, статистика
│   ├── storage.go        — Load / SaveNow / rotateBackup
│   ├── plants.go         — CRUD растений
│   ├── sales.go          — продажи со списанием склада
│   ├── projects.go       — проекты озеленения (атомарное списание)
│   ├── expenses.go       — расходы
│   ├── employees.go      — сотрудники
│   ├── stats.go          — агрегаты за период
│   ├── import_csv.go     — импорт прайс-листа из Google Sheets
│   └── *_test.go         — 19+ unit-тестов
└── api/                  — HTTP-обработчики (по файлу на ресурс)
static/
└── index.html            — SPA в одном файле (vanilla JS, нулевые зависимости)
```

**Технические решения:**

- **Атомарная запись** — `data.json.tmp` + `os.Rename`, файл не повредится при крэше в момент сохранения
- **Бэкапы с ротацией** — раз в час snapshot в `backups/`, держим 30 последних
- **`log/slog`** — структурное логирование, стандарт Go 1.21+
- **`sync.RWMutex`** — конкурентный доступ без гонок, отдельные RLock для чтения
- **Graceful Shutdown** — сервер дожидается in-flight запросов и финально сохраняет данные при `SIGTERM`/`SIGINT`
- **Нулевые зависимости** — только стандартная библиотека, `go.sum` пустой
- **Покрытие тестами** — 46% в `internal/storage` (ключевая бизнес-логика)

---

## API

| Метод    | Путь                | Описание                           |
|----------|---------------------|------------------------------------|
| `GET`    | `/api/plants`       | Список растений                    |
| `POST`   | `/api/plants`       | Добавить растение                  |
| `PUT`    | `/api/plants/:id`   | Обновить растение                  |
| `DELETE` | `/api/plants/:id`   | Удалить растение                   |
| `GET`    | `/api/sales`        | История продаж                     |
| `POST`   | `/api/sales`        | Добавить продажу (с автосписанием) |
| `GET`    | `/api/expenses`     | Список расходов                    |
| `POST`   | `/api/expenses`     | Добавить расход                    |
| `GET`    | `/api/projects`     | Список проектов                    |
| `POST`   | `/api/projects`     | Создать проект (с автосписанием)   |
| `GET`    | `/api/stats`        | Дашборд: прибыль, топ-5, каналы    |
| `GET`    | `/api/salary`       | Расчёт зарплат по % от прибыли     |
| `POST`   | `/api/import/csv`   | Импорт прайс-листа из CSV          |

---

## Стек

| Слой     | Технология                                      |
|----------|-------------------------------------------------|
| Backend  | Go 1.21 · net/http · encoding/json · log/slog  |
| Frontend | Vanilla HTML / CSS / JavaScript                 |
| Storage  | JSON-файл (data.json)                           |
| Deps     | 0                                               |

---

## Автор

**Aliaksandr Kacheuski** — студент БГУИР (инженер по инфокоммуникациям), изучаю Go.  
Этот инструмент написан для реального бизнеса и используется в повседневной работе.

[github.com/cacarerru-prog](https://github.com/cacarerru-prog)
