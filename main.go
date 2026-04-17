package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ─── МОДЕЛИ ───────────────────────────────────────────────

type Plant struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Size     string  `json:"size"`
	Price    float64 `json:"price"`
	Qty      int     `json:"qty"`
}

type Sale struct {
	ID        int     `json:"id"`
	PlantName string  `json:"plant_name"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	Total     float64 `json:"total"`
	Channel   string  `json:"channel"`
	Date      string  `json:"date"`
}

type Expense struct {
	ID          int     `json:"id"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
}

type Employee struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

// ProjectPlant — одна строка в проекте озеленения: какое растение, сколько, по какой цене.
// Аналогия: строка в счёте/смете для клиента.
type ProjectPlant struct {
	PlantName string  `json:"plant_name"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	Total     float64 `json:"total"`
}

// Project — проект озеленения (например, "Кафе Весна, апрель 2026").
// Отличается от обычной продажи тем, что:
//   - привязан к конкретному заведению (Client)
//   - содержит список использованных растений (Plants)
//   - включает стоимость работы (LaborCost)
//   - при создании автоматически списывает растения со склада
type Project struct {
	ID        int            `json:"id"`
	Client    string         `json:"client"`     // название заведения
	Date      string         `json:"date"`
	Channel   string         `json:"channel"`    // канал привлечения клиента
	Plants    []ProjectPlant `json:"plants"`     // список использованных растений
	LaborCost float64        `json:"labor_cost"` // стоимость работы (монтаж, уход и т.д.)
	Total     float64        `json:"total"`      // итоговая сумма (растения + работа)
	Notes     string         `json:"notes"`      // заметки (пожелания клиента и т.д.)
}

type DB struct {
	Plants    []Plant    `json:"plants"`
	Sales     []Sale     `json:"sales"`
	Expenses  []Expense  `json:"expenses"`
	Employees []Employee `json:"employees"`
	Projects  []Project  `json:"projects"` // ← новый раздел
	NextID    int        `json:"next_id"`
}

// ─── ХРАНИЛИЩЕ ────────────────────────────────────────────

const dataFile = "data.json"

var (
	db DB
	mu sync.RWMutex
)

func defaultEmployees() []Employee {
	return []Employee{
		{ID: 1, Name: "Елена", Percent: 50},
		{ID: 2, Name: "Александр", Percent: 25},
		{ID: 3, Name: "Данила", Percent: 25},
	}
}

func loadDB() error {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("data.json не найден, создаём новую базу")
			db = DB{NextID: 1, Employees: defaultEmployees()}
			return nil
		}
		return fmt.Errorf("loadDB: не удалось прочитать файл: %w", err)
	}
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("loadDB: файл data.json повреждён: %w", err)
	}
	if db.NextID == 0 {
		db.NextID = 1
	}
	if len(db.Employees) == 0 {
		db.Employees = defaultEmployees()
	}
	slog.Info("база данных загружена",
		"plants", len(db.Plants),
		"sales", len(db.Sales),
		"expenses", len(db.Expenses),
		"projects", len(db.Projects),
	)
	return nil
}

func saveDB() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("saveDB: marshal: %w", err)
	}
	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		return fmt.Errorf("saveDB: write: %w", err)
	}
	return nil
}

func nextID() int {
	id := db.NextID
	db.NextID++
	return id
}

// ─── HELPERS ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON failed", "err", err)
	}
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func idFromPath(path, prefix string) (int, error) {
	s := strings.TrimPrefix(path, prefix)
	s = strings.Trim(s, "/")
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("невалидный ID %q: %w", s, err)
	}
	return id, nil
}

func today() string {
	return time.Now().Format("02.01.2006")
}

// extractMonthYear вытаскивает месяц и год из строки формата "dd.mm.yyyy".
// Пример: "15.04.2026" → month=4, year=2026
func extractMonthYear(date string) (month, year int, err error) {
	parts := strings.Split(date, ".")
	if len(parts) != 3 {
		return 0, 0, fmt.Errorf("неверный формат даты: %q", date)
	}
	month, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("неверный месяц в дате %q: %w", date, err)
	}
	year, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, fmt.Errorf("неверный год в дате %q: %w", date, err)
	}
	return month, year, nil
}

// matchesPeriod проверяет, попадает ли дата в нужный месяц и год.
// Если дата кривая — запись просто не попадает в статистику (не крашим сервер).
func matchesPeriod(date string, month, year int) bool {
	m, y, err := extractMonthYear(date)
	if err != nil {
		return false
	}
	return m == month && y == year
}

// Названия месяцев на русском — для читаемых заголовков
var monthNames = [13]string{
	"", "Январь", "Февраль", "Март", "Апрель",
	"Май", "Июнь", "Июль", "Август",
	"Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

// ─── ОБРАБОТЧИКИ: РАСТЕНИЯ ────────────────────────────────

func handlePlants(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/plants" {
		mu.RLock()
		plants := make([]Plant, len(db.Plants))
		copy(plants, db.Plants)
		mu.RUnlock()
		if plants == nil {
			writeJSON(w, []Plant{})
		} else {
			writeJSON(w, plants)
		}
		return
	}

	if r.Method == http.MethodPost {
		var p Plant
		if err := readJSON(r, &p); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		p.ID = nextID()
		db.Plants = append(db.Plants, p)
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("POST /api/plants: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, p)
		return
	}

	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/plants/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, p := range db.Plants {
			if p.ID == id {
				db.Plants = append(db.Plants[:i], db.Plants[i+1:]...)
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("DELETE /api/plants: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}

	if r.Method == http.MethodPut {
		id, err := idFromPath(r.URL.Path, "/api/plants/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var p Plant
		if err := readJSON(r, &p); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, pl := range db.Plants {
			if pl.ID == id {
				p.ID = id
				db.Plants[i] = p
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("PUT /api/plants: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ОБРАБОТЧИКИ: ПРОДАЖИ (розничные) ─────────────────────

func handleSales(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/sales" {
		mu.RLock()
		s := make([]Sale, len(db.Sales))
		copy(s, db.Sales)
		mu.RUnlock()
		if s == nil {
			writeJSON(w, []Sale{})
			return
		}
		result := make([]Sale, len(s))
		for i, v := range s {
			result[len(s)-1-i] = v
		}
		if len(result) > 100 {
			result = result[:100]
		}
		writeJSON(w, result)
		return
	}

	if r.Method == http.MethodPost {
		var s Sale
		if err := readJSON(r, &s); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		var plant *Plant
		for i := range db.Plants {
			if strings.EqualFold(db.Plants[i].Name, s.PlantName) {
				plant = &db.Plants[i]
				break
			}
		}
		if plant == nil {
			mu.Unlock()
			http.Error(w, "Растение не найдено на складе", http.StatusBadRequest)
			return
		}
		if s.Qty > plant.Qty {
			mu.Unlock()
			http.Error(w, fmt.Sprintf("На складе только %d шт.", plant.Qty), http.StatusBadRequest)
			return
		}
		s.ID = nextID()
		s.Total = float64(s.Qty) * s.Price
		if s.Date == "" {
			s.Date = today()
		}
		plant.Qty -= s.Qty
		db.Sales = append(db.Sales, s)
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("POST /api/sales: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		slog.Info("продажа", "plant", s.PlantName, "qty", s.Qty, "total", s.Total)
		writeJSON(w, s)
		return
	}

	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/sales/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, s := range db.Sales {
			if s.ID == id {
				for j, p := range db.Plants {
					if strings.EqualFold(p.Name, s.PlantName) {
						db.Plants[j].Qty += s.Qty
						break
					}
				}
				db.Sales = append(db.Sales[:i], db.Sales[i+1:]...)
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("DELETE /api/sales: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ОБРАБОТЧИКИ: РАСХОДЫ ─────────────────────────────────

func handleExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/expenses" {
		mu.RLock()
		e := make([]Expense, len(db.Expenses))
		copy(e, db.Expenses)
		mu.RUnlock()
		if e == nil {
			writeJSON(w, []Expense{})
			return
		}
		result := make([]Expense, len(e))
		for i, v := range e {
			result[len(e)-1-i] = v
		}
		if len(result) > 100 {
			result = result[:100]
		}
		writeJSON(w, result)
		return
	}

	if r.Method == http.MethodPost {
		var e Expense
		if err := readJSON(r, &e); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		e.ID = nextID()
		if e.Date == "" {
			e.Date = today()
		}
		db.Expenses = append(db.Expenses, e)
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("POST /api/expenses: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, e)
		return
	}

	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/expenses/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, e := range db.Expenses {
			if e.ID == id {
				db.Expenses = append(db.Expenses[:i], db.Expenses[i+1:]...)
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("DELETE /api/expenses: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ОБРАБОТЧИКИ: ПРОЕКТЫ ОЗЕЛЕНЕНИЯ ─────────────────────

func handleProjects(w http.ResponseWriter, r *http.Request) {
	// GET /api/projects — список проектов (последние 50, в обратном порядке)
	if r.Method == http.MethodGet && r.URL.Path == "/api/projects" {
		mu.RLock()
		src := make([]Project, len(db.Projects))
		copy(src, db.Projects)
		mu.RUnlock()

		result := make([]Project, len(src))
		for i, v := range src {
			result[len(src)-1-i] = v
		}
		if len(result) > 50 {
			result = result[:50]
		}
		writeJSON(w, result)
		return
	}

	// POST /api/projects — новый проект
	if r.Method == http.MethodPost {
		var proj Project
		if err := readJSON(r, &proj); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if proj.Client == "" {
			http.Error(w, "Укажи название заведения (client)", http.StatusBadRequest)
			return
		}
		if len(proj.Plants) == 0 && proj.LaborCost == 0 {
			http.Error(w, "Проект должен содержать растения или стоимость работы", http.StatusBadRequest)
			return
		}

		mu.Lock()

		// Проверяем наличие всех растений перед списанием.
		// Аналогия: сначала смотрим, всё ли есть на складе, и только потом
		// выдаём товар. Не выдаём частично — или всё или ничего.
		for _, pp := range proj.Plants {
			var found *Plant
			for i := range db.Plants {
				if strings.EqualFold(db.Plants[i].Name, pp.PlantName) {
					found = &db.Plants[i]
					break
				}
			}
			if found == nil {
				mu.Unlock()
				http.Error(w, fmt.Sprintf("Растение %q не найдено на складе", pp.PlantName), http.StatusBadRequest)
				return
			}
			if pp.Qty > found.Qty {
				mu.Unlock()
				http.Error(w, fmt.Sprintf("Растение %q: на складе только %d шт., запрошено %d", pp.PlantName, found.Qty, pp.Qty), http.StatusBadRequest)
				return
			}
		}

		// Всё есть — списываем и считаем итог
		proj.Total = proj.LaborCost
		for i := range proj.Plants {
			proj.Plants[i].Total = float64(proj.Plants[i].Qty) * proj.Plants[i].Price
			proj.Total += proj.Plants[i].Total

			// Списываем с остатков
			for j := range db.Plants {
				if strings.EqualFold(db.Plants[j].Name, proj.Plants[i].PlantName) {
					db.Plants[j].Qty -= proj.Plants[i].Qty
					break
				}
			}
		}

		proj.ID = nextID()
		if proj.Date == "" {
			proj.Date = today()
		}
		db.Projects = append(db.Projects, proj)

		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("POST /api/projects: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()

		slog.Info("проект создан", "client", proj.Client, "total", proj.Total, "plants", len(proj.Plants))
		writeJSON(w, proj)
		return
	}

	// DELETE /api/projects/{id} — удаляем проект, возвращаем растения на склад
	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/projects/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, proj := range db.Projects {
			if proj.ID == id {
				// Возвращаем все растения обратно на склад
				for _, pp := range proj.Plants {
					for j := range db.Plants {
						if strings.EqualFold(db.Plants[j].Name, pp.PlantName) {
							db.Plants[j].Qty += pp.Qty
							break
						}
					}
				}
				db.Projects = append(db.Projects[:i], db.Projects[i+1:]...)
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("DELETE /api/projects: сохранение", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ИМПОРТ CSV ───────────────────────────────────────────

func handleImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "метод не разрешён", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "ошибка парсинга формы: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "файл не найден", http.StatusBadRequest)
		return
	}
	defer file.Close()

	category := r.FormValue("category")
	if category == "" {
		category = "Лиственные"
	}

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	imported, skipped, lineNum := 0, 0, 0
	mu.Lock()
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		lineNum++
		if len(record) < 4 {
			continue
		}
		name := strings.TrimSpace(record[1])
		if name == "" || name == "Наименование" || strings.HasPrefix(name, "ИТОГО") {
			continue
		}
		if lineNum == 1 {
			continue
		}
		size := strings.TrimSpace(record[2])
		priceStr := strings.TrimSpace(record[3])
		qtyStr := ""
		if len(record) >= 5 {
			qtyStr = strings.TrimSpace(record[4])
		}
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			skipped++
			continue
		}
		qty := 0
		if qtyStr != "" {
			qty, _ = strconv.Atoi(qtyStr)
		}
		found := false
		for i, p := range db.Plants {
			if strings.EqualFold(p.Name, name) {
				db.Plants[i].Price = price
				db.Plants[i].Qty = qty
				db.Plants[i].Size = size
				found = true
				imported++
				break
			}
		}
		if !found {
			db.Plants = append(db.Plants, Plant{
				ID: nextID(), Name: name, Category: category,
				Size: size, Price: price, Qty: qty,
			})
			imported++
		}
	}
	if err := saveDB(); err != nil {
		mu.Unlock()
		slog.Error("import CSV: сохранение", "err", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	mu.Unlock()
	slog.Info("CSV импорт", "imported", imported, "skipped", skipped)
	writeJSON(w, map[string]any{"imported": imported, "skipped": skipped, "total": len(db.Plants)})
}

// ─── СОТРУДНИКИ ───────────────────────────────────────────

func handleEmployees(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		mu.RLock()
		emps := make([]Employee, len(db.Employees))
		copy(emps, db.Employees)
		mu.RUnlock()
		writeJSON(w, emps)
		return
	}
	if r.Method == http.MethodPost {
		var e Employee
		if err := readJSON(r, &e); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		e.ID = nextID()
		db.Employees = append(db.Employees, e)
		if err := saveDB(); err != nil {
			mu.Unlock()
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, e)
		return
	}
	if r.Method == http.MethodPut {
		id, err := idFromPath(r.URL.Path, "/api/employees/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var e Employee
		if err := readJSON(r, &e); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, emp := range db.Employees {
			if emp.ID == id {
				e.ID = id
				db.Employees[i] = e
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/employees/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		for i, e := range db.Employees {
			if e.ID == id {
				db.Employees = append(db.Employees[:i], db.Employees[i+1:]...)
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── СТАТИСТИКА (с фильтрацией по периоду) ────────────────

type TopPlant struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
}

type SalaryStat struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
	Amount  float64 `json:"amount"`
}

type Stats struct {
	// Информация о периоде
	PeriodMonth int    `json:"period_month"` // номер месяца: 4
	PeriodYear  int    `json:"period_year"`  // год: 2026
	PeriodLabel string `json:"period_label"` // "Апрель 2026"

	// Финансы за период
	SalesRevenue    float64 `json:"sales_revenue"`    // выручка от розничных продаж
	ProjectsRevenue float64 `json:"projects_revenue"` // выручка от проектов озеленения
	TotalRevenue    float64 `json:"total_revenue"`     // итого выручка
	TotalExpenses   float64 `json:"total_expenses"`    // расходы
	Profit          float64 `json:"profit"`            // прибыль до зарплат
	StockValue      float64 `json:"stock_value"`       // стоимость остатков (всегда полная, не за период)

	ByChannel     map[string]float64 `json:"by_channel"`
	TopPlants     []TopPlant         `json:"top_plants"`
	Salaries      []SalaryStat       `json:"salaries"`
	TotalSalaries float64            `json:"total_salaries"`
	NetProfit     float64            `json:"net_profit"`

	// Количество событий за период — для информации
	SalesCount    int `json:"sales_count"`
	ProjectsCount int `json:"projects_count"`
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	// Читаем query-параметры: /api/stats?month=4&year=2026
	// Если не переданы — используем текущий месяц.
	//
	// r.URL.Query() — это как словарь параметров из адресной строки.
	// r.URL.Query().Get("month") вернёт строку "4" или "" если нет параметра.
	now := time.Now()
	filterMonth := int(now.Month())
	filterYear := now.Year()

	if m, err := strconv.Atoi(r.URL.Query().Get("month")); err == nil && m >= 1 && m <= 12 {
		filterMonth = m
	}
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil && y >= 2020 && y <= 2100 {
		filterYear = y
	}

	// Копируем данные под замком, а считаем уже без него
	mu.RLock()
	sales := make([]Sale, len(db.Sales))
	copy(sales, db.Sales)
	expenses := make([]Expense, len(db.Expenses))
	copy(expenses, db.Expenses)
	plants := make([]Plant, len(db.Plants))
	copy(plants, db.Plants)
	employees := make([]Employee, len(db.Employees))
	copy(employees, db.Employees)
	projects := make([]Project, len(db.Projects))
	copy(projects, db.Projects)
	mu.RUnlock()

	stats := Stats{
		PeriodMonth: filterMonth,
		PeriodYear:  filterYear,
		PeriodLabel: fmt.Sprintf("%s %d", monthNames[filterMonth], filterYear),
		ByChannel:   make(map[string]float64),
		TopPlants:   []TopPlant{},
		Salaries:    []SalaryStat{},
	}

	plantTotals := make(map[string]float64)

	// Считаем только продажи за нужный месяц
	for _, s := range sales {
		if !matchesPeriod(s.Date, filterMonth, filterYear) {
			continue
		}
		stats.SalesRevenue += s.Total
		stats.ByChannel[s.Channel] += s.Total
		plantTotals[s.PlantName] += s.Total
		stats.SalesCount++
	}

	// Считаем проекты озеленения за нужный месяц
	for _, proj := range projects {
		if !matchesPeriod(proj.Date, filterMonth, filterYear) {
			continue
		}
		stats.ProjectsRevenue += proj.Total
		stats.ByChannel[proj.Channel] += proj.Total
		// Учитываем растения из проектов в топе
		for _, pp := range proj.Plants {
			plantTotals[pp.PlantName] += pp.Total
		}
		stats.ProjectsCount++
	}

	// Считаем расходы за нужный месяц
	for _, e := range expenses {
		if !matchesPeriod(e.Date, filterMonth, filterYear) {
			continue
		}
		stats.TotalExpenses += e.Amount
	}

	// Стоимость склада — всегда актуальная (не за период)
	for _, p := range plants {
		stats.StockValue += p.Price * float64(p.Qty)
	}

	stats.TotalRevenue = stats.SalesRevenue + stats.ProjectsRevenue
	stats.Profit = stats.TotalRevenue - stats.TotalExpenses

	// Зарплаты = % от прибыли текущего периода
	for _, emp := range employees {
		amount := stats.Profit * emp.Percent / 100
		stats.Salaries = append(stats.Salaries, SalaryStat{
			Name:    emp.Name,
			Percent: emp.Percent,
			Amount:  amount,
		})
		stats.TotalSalaries += amount
	}
	stats.NetProfit = stats.Profit - stats.TotalSalaries

	// Топ-5 растений (пузырьковая сортировка)
	for name, total := range plantTotals {
		stats.TopPlants = append(stats.TopPlants, TopPlant{name, total})
	}
	for i := 0; i < len(stats.TopPlants)-1; i++ {
		for j := i + 1; j < len(stats.TopPlants); j++ {
			if stats.TopPlants[j].Total > stats.TopPlants[i].Total {
				stats.TopPlants[i], stats.TopPlants[j] = stats.TopPlants[j], stats.TopPlants[i]
			}
		}
	}
	if len(stats.TopPlants) > 5 {
		stats.TopPlants = stats.TopPlants[:5]
	}

	writeJSON(w, stats)
}

// ─── РОУТЕР ───────────────────────────────────────────────

func router(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	slog.Debug("запрос", "method", r.Method, "path", r.URL.Path)

	path := r.URL.Path
	switch {
	case strings.HasPrefix(path, "/api/plants"):
		handlePlants(w, r)
	case strings.HasPrefix(path, "/api/sales"):
		handleSales(w, r)
	case strings.HasPrefix(path, "/api/expenses"):
		handleExpenses(w, r)
	case strings.HasPrefix(path, "/api/employees"):
		handleEmployees(w, r)
	case strings.HasPrefix(path, "/api/projects"): // ← новый маршрут
		handleProjects(w, r)
	case path == "/api/stats":
		handleStats(w, r)
	case path == "/api/import/csv":
		handleImportCSV(w, r)
	default:
		http.FileServer(http.Dir("./static")).ServeHTTP(w, r)
	}
}

// ─── MAIN ─────────────────────────────────────────────────

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	if err := loadDB(); err != nil {
		slog.Error("не удалось загрузить базу данных", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{Addr: ":8080", Handler: http.DefaultServeMux}
	http.HandleFunc("/", router)

	go func() {
		fmt.Println("====================================")
		fmt.Println("  Растения — учёт запущен!")
		fmt.Println("  http://localhost:8080")
		fmt.Println("  Для остановки: Ctrl+C")
		fmt.Println("====================================")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("сервер упал", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("завершаем работу...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("ошибка остановки", "err", err)
	}
	mu.Lock()
	if err := saveDB(); err != nil {
		slog.Error("ошибка сохранения при выходе", "err", err)
	}
	mu.Unlock()
	slog.Info("готово. До свидания!")
}
