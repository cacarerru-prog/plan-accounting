package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"  // структурное логирование — стандарт Go 1.21+
	"net/http"
	"os"
	"os/signal"  // слушаем сигналы ОС (Ctrl+C = SIGINT)
	"strconv"
	"strings"
	"sync"      // пакет для синхронизации горутин (параллельных потоков)
	"syscall"   // константы сигналов: SIGINT, SIGTERM
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

type DB struct {
	Plants    []Plant    `json:"plants"`
	Sales     []Sale     `json:"sales"`
	Expenses  []Expense  `json:"expenses"`
	Employees []Employee `json:"employees"`
	NextID    int        `json:"next_id"`
}

// ─── ХРАНИЛИЩЕ ────────────────────────────────────────────

const dataFile = "data.json"

var (
	db DB

	// 🔑 ФИКС #1: sync.RWMutex — "замок" на нашу базу данных.
	//
	// Аналогия: представь, что db — это тетрадь на кухне.
	// Если два человека пишут в неё одновременно — записи перемешаются.
	// mu — это правило: "пишет только один, остальные ждут".
	//
	// RWMutex = Read-Write Mutex. Два режима:
	//   mu.Lock()   / mu.Unlock()   — эксклюзивный замок для ЗАПИСИ (никто не может ни читать, ни писать)
	//   mu.RLock()  / mu.RUnlock()  — разделённый замок для ЧТЕНИЯ (читать могут все, но писать — никто)
	mu sync.RWMutex
)

// defaultEmployees возвращает список сотрудников по умолчанию.
// Вынесли в отдельную функцию, чтобы не дублировать код в loadDB.
func defaultEmployees() []Employee {
	return []Employee{
		{ID: 1, Name: "Елена", Percent: 50},
		{ID: 2, Name: "Александр", Percent: 25},
		{ID: 3, Name: "Данила", Percent: 25},
	}
}

// 🔑 ФИКС #2: loadDB теперь возвращает error — не молчит, если что-то пошло не так.
//
// Было:   func loadDB() { ... json.Unmarshal(data, &db) ... }
// Стало:  func loadDB() error { ... if err := json.Unmarshal(...); err != nil { return err } ... }
//
// Аналогия: раньше повар читал рецепт и, не найдя его, молча начинал готовить
// по памяти. Теперь он выходит и говорит: "Рецепта нет — что делать?"
func loadDB() error {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		// os.IsNotExist(err) == true означает: файл просто ещё не создан (первый запуск).
		// Это не ошибка! Создаём пустую базу и продолжаем.
		if os.IsNotExist(err) {
			slog.Info("data.json не найден, создаём новую базу")
			db = DB{
				NextID:    1,
				Employees: defaultEmployees(),
			}
			return nil // nil в Go = "ошибки нет, всё хорошо"
		}
		// Любая другая ошибка чтения — это проблема (нет прав, диск сломан и т.д.)
		return fmt.Errorf("loadDB: не удалось прочитать файл: %w", err)
		// %w — это "обёртка" ошибки. Позволяет потом проверить: errors.Is(err, originalErr)
	}

	// json.Unmarshal — переводит JSON-текст в Go-структуру.
	// Раньше ошибка тут просто игнорировалась! Если файл битый — db оставался пустым.
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("loadDB: файл data.json повреждён (не валидный JSON): %w", err)
	}

	// Защита от нулевого ID (если старый файл не имел этого поля)
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
	)
	return nil
}

// 🔑 ФИКС #2 (продолжение): saveDB теперь возвращает error.
//
// Было:   func saveDB() { data, _ := json.MarshalIndent... os.WriteFile... }
// Стало:  func saveDB() error { ... if err != nil { return err } ... }
//
// ВАЖНО: saveDB всегда вызывается внутри mu.Lock() — замок уже захвачен,
// поэтому здесь мы НЕ захватываем его снова (это привело бы к дедлоку — вечному ожиданию).
func saveDB() error {
	// json.MarshalIndent — переводит Go-структуру в красивый JSON с отступами.
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		// В реальности MarshalIndent почти никогда не падает с ошибкой,
		// но обрабатываем на всякий случай — правила есть правила.
		return fmt.Errorf("saveDB: не удалось сериализовать данные: %w", err)
	}

	// os.WriteFile — атомарно записывает файл (сначала во временный, потом переименовывает).
	// 0644 — права доступа: владелец читает/пишет, остальные только читают.
	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		return fmt.Errorf("saveDB: не удалось записать файл: %w", err)
	}
	return nil
}

func nextID() int {
	// Эта функция ВСЕГДА вызывается внутри mu.Lock() — замок уже есть.
	id := db.NextID
	db.NextID++
	return id
}

// ─── HELPERS ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Если не смогли отправить JSON — логируем. Клиент уже получил часть ответа,
		// поэтому http.Error здесь не поможет, просто фиксируем факт.
		slog.Error("writeJSON: не удалось записать ответ", "err", err)
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
		return 0, fmt.Errorf("idFromPath: невалидный ID %q: %w", s, err)
	}
	return id, nil
}

func today() string {
	return time.Now().Format("02.01.2006")
}

// ─── ОБРАБОТЧИКИ: РАСТЕНИЯ ────────────────────────────────

func handlePlants(w http.ResponseWriter, r *http.Request) {
	// GET /api/plants — читаем список растений
	if r.Method == http.MethodGet && r.URL.Path == "/api/plants" {
		mu.RLock() // замок для чтения: другие тоже могут читать одновременно
		plants := db.Plants
		mu.RUnlock()

		if plants == nil {
			writeJSON(w, []Plant{})
		} else {
			writeJSON(w, plants)
		}
		return
	}

	// POST /api/plants — добавляем новое растение
	if r.Method == http.MethodPost {
		var p Plant
		if err := readJSON(r, &p); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock() // эксклюзивный замок для записи: все остальные ждут
		p.ID = nextID()
		db.Plants = append(db.Plants, p)
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("POST /api/plants: не удалось сохранить", "err", err)
			http.Error(w, "Ошибка сервера: не удалось сохранить данные", http.StatusInternalServerError)
			return
		}
		mu.Unlock()

		writeJSON(w, p)
		return
	}

	// DELETE /api/plants/{id}
	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/plants/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		for i, p := range db.Plants {
			if p.ID == id {
				// Трюк удаления из слайса: заменяем элемент i последним, обрезаем хвост.
				// Аналогия: убираем стул из середины ряда, пересаживая последнего гостя на его место.
				db.Plants = append(db.Plants[:i], db.Plants[i+1:]...)
				break
			}
		}
		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("DELETE /api/plants: не удалось сохранить", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()

		writeJSON(w, map[string]bool{"ok": true})
		return
	}

	// PUT /api/plants/{id} — обновляем растение
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
			slog.Error("PUT /api/plants: не удалось сохранить", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()

		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ОБРАБОТЧИКИ: ПРОДАЖИ ─────────────────────────────────

func handleSales(w http.ResponseWriter, r *http.Request) {
	// GET /api/sales — последние 100 продаж в обратном порядке
	if r.Method == http.MethodGet && r.URL.Path == "/api/sales" {
		mu.RLock()
		s := make([]Sale, len(db.Sales))
		copy(s, db.Sales) // копируем, чтобы не держать замок пока сортируем
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

	// POST /api/sales — новая продажа
	if r.Method == http.MethodPost {
		var s Sale
		if err := readJSON(r, &s); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock() // захватываем замок — будем и читать и писать

		// Ищем растение на складе
		var plant *Plant
		for i := range db.Plants {
			if strings.EqualFold(db.Plants[i].Name, s.PlantName) {
				plant = &db.Plants[i] // берём указатель ("ссылку на оригинал"), чтобы потом изменить Qty
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

		plant.Qty -= s.Qty // списываем с остатка через указатель — меняем оригинал
		db.Sales = append(db.Sales, s)

		if err := saveDB(); err != nil {
			mu.Unlock()
			slog.Error("POST /api/sales: не удалось сохранить", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()

		slog.Info("продажа добавлена",
			"plant", s.PlantName,
			"qty", s.Qty,
			"total", s.Total,
			"channel", s.Channel,
		)
		writeJSON(w, s)
		return
	}

	// DELETE /api/sales/{id} — удаляем продажу, возвращаем qty на склад
	if r.Method == http.MethodDelete {
		id, err := idFromPath(r.URL.Path, "/api/sales/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		for i, s := range db.Sales {
			if s.ID == id {
				// Возвращаем количество обратно на склад
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
			slog.Error("DELETE /api/sales: не удалось сохранить", "err", err)
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
		// Раньше тут ошибка readJSON молча игнорировалась — исправили!
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
			slog.Error("POST /api/expenses: не удалось сохранить", "err", err)
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
			slog.Error("DELETE /api/expenses: не удалось сохранить", "err", err)
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

	// ParseMultipartForm — разбираем форму с файлом.
	// 10 << 20 = 10 МБ — максимальный размер в памяти.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "ошибка парсинга формы: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "файл не найден в запросе", http.StatusBadRequest)
		return
	}
	defer file.Close() // defer — вызовется когда функция завершится, чтобы закрыть файл

	category := r.FormValue("category")
	if category == "" {
		category = "Лиственные"
	}

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	imported := 0
	skipped := 0
	lineNum := 0

	mu.Lock() // захватываем замок на всё время импорта
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
				ID:       nextID(),
				Name:     name,
				Category: category,
				Size:     size,
				Price:    price,
				Qty:      qty,
			})
			imported++
		}
	}

	if err := saveDB(); err != nil {
		mu.Unlock()
		slog.Error("import CSV: не удалось сохранить", "err", err)
		http.Error(w, "Ошибка сервера при сохранении", http.StatusInternalServerError)
		return
	}
	mu.Unlock()

	slog.Info("CSV импортирован", "imported", imported, "skipped", skipped)
	writeJSON(w, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"total":    len(db.Plants),
	})
}

// ─── СОТРУДНИКИ И ЗАРПЛАТЫ ────────────────────────────────

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
			slog.Error("POST /api/employees: не удалось сохранить", "err", err)
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
			slog.Error("PUT /api/employees: не удалось сохранить", "err", err)
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
			slog.Error("DELETE /api/employees: не удалось сохранить", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── СТАТИСТИКА ───────────────────────────────────────────

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
	TotalRevenue  float64            `json:"total_revenue"`
	TotalExpenses float64            `json:"total_expenses"`
	Profit        float64            `json:"profit"`
	StockValue    float64            `json:"stock_value"`
	ByChannel     map[string]float64 `json:"by_channel"`
	TopPlants     []TopPlant         `json:"top_plants"`
	Salaries      []SalaryStat       `json:"salaries"`
	TotalSalaries float64            `json:"total_salaries"`
	NetProfit     float64            `json:"net_profit"`
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	mu.RLock() // только читаем — используем RLock
	// Копируем нужные данные под замком
	sales := make([]Sale, len(db.Sales))
	copy(sales, db.Sales)
	expenses := make([]Expense, len(db.Expenses))
	copy(expenses, db.Expenses)
	plants := make([]Plant, len(db.Plants))
	copy(plants, db.Plants)
	employees := make([]Employee, len(db.Employees))
	copy(employees, db.Employees)
	mu.RUnlock()
	// Вычисляем статистику уже без замка — только читаем локальные копии

	stats := Stats{
		ByChannel: make(map[string]float64),
		TopPlants: []TopPlant{},
		Salaries:  []SalaryStat{},
	}

	for _, s := range sales {
		stats.TotalRevenue += s.Total
		stats.ByChannel[s.Channel] += s.Total
	}
	for _, e := range expenses {
		stats.TotalExpenses += e.Amount
	}
	for _, p := range plants {
		stats.StockValue += p.Price * float64(p.Qty)
	}
	stats.Profit = stats.TotalRevenue - stats.TotalExpenses

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

	plantTotals := make(map[string]float64)
	for _, s := range sales {
		plantTotals[s.PlantName] += s.Total
	}
	for name, total := range plantTotals {
		stats.TopPlants = append(stats.TopPlants, TopPlant{name, total})
	}
	// Сортировка пузырьком (bubble sort) — топ-5 по выручке
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

	// Логируем каждый запрос — удобно при отладке
	slog.Debug("входящий запрос", "method", r.Method, "path", r.URL.Path)

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
	// 🔑 ФИКС #3а: Настраиваем slog — структурное логирование.
	//
	// slog.NewTextHandler — пишет логи в текстовом формате в os.Stdout (консоль).
	// Каждая строка лога будет выглядеть так:
	//   time=2025-04-17T10:00:00Z level=INFO msg="сервер запущен" addr=:8080
	//
	// slog.LevelDebug — показывать ВСЕ уровни логов (Debug, Info, Warn, Error).
	// В продакшене обычно ставят slog.LevelInfo чтобы не засорять вывод.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger) // делаем его логером по умолчанию для всего приложения

	// Загружаем базу данных. Теперь мы ОБЯЗАНЫ проверить ошибку.
	if err := loadDB(); err != nil {
		// slog.Error — самый высокий уровень. os.Exit(1) — немедленный выход с кодом ошибки.
		slog.Error("не удалось загрузить базу данных", "err", err)
		os.Exit(1) // 1 = "программа завершилась с ошибкой" (0 = успех)
	}

	// 🔑 ФИКС #3б: Graceful Shutdown — "мягкое завершение".
	//
	// Аналогия: обычный выход — это рвануть скатерть со стола.
	// Graceful Shutdown — это сказать официантам "заканчивайте текущие заказы,
	// новых не принимайте, через 5 секунд закрываемся".
	//
	// Создаём http.Server явно (раньше был просто http.ListenAndServe — без возможности остановить).
	srv := &http.Server{
		Addr:    ":8080",
		Handler: http.DefaultServeMux, // используем стандартный мультиплексор
	}

	http.HandleFunc("/", router)

	// Запускаем сервер в отдельной горутине (горутина = лёгкий параллельный поток в Go).
	// Аналогия: нанимаем помощника и говорим "работай в фоне, я займусь другим".
	// go func() { ... }() — это и есть запуск горутины.
	go func() {
		slog.Info("сервер запущен",
			"addr", "http://localhost:8080",
			"data", dataFile,
		)
		fmt.Println("====================================")
		fmt.Println("  Растения — учёт запущен!")
		fmt.Println("  http://localhost:8080")
		fmt.Println("  Для остановки: Ctrl+C")
		fmt.Println("====================================")

		// srv.ListenAndServe() блокирует горутину — слушает запросы бесконечно.
		// http.ErrServerClosed — это НЕ ошибка, это сигнал что мы сами вызвали srv.Shutdown().
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("сервер упал с ошибкой", "err", err)
			os.Exit(1)
		}
	}()

	// Создаём канал (channel) для приёма сигналов от ОС.
	// Канал в Go — это как труба: с одного конца кладут, с другого забирают.
	// make(chan os.Signal, 1) — труба с буфером на 1 сигнал (не блокирует отправителя).
	quit := make(chan os.Signal, 1)

	// signal.Notify говорит ОС: "когда получишь SIGINT (Ctrl+C) или SIGTERM (kill),
	// положи это в канал quit".
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// <-quit — БЛОКИРУЕМСЯ здесь и ждём, пока в канал что-то не придёт.
	// Аналогия: стоим у двери и ждём звонка. Пришёл сигнал — идём дальше.
	<-quit

	slog.Info("получен сигнал остановки, завершаем работу...")

	// Даём серверу 5 секунд на завершение текущих запросов.
	// context.WithTimeout — это таймер: "через 5 секунд сдаёмся".
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // defer — вызовется в конце main(), чтобы освободить ресурсы таймера

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("ошибка при остановке сервера", "err", err)
	}

	// Финальное сохранение перед выходом — данные точно не потеряются.
	mu.Lock()
	if err := saveDB(); err != nil {
		slog.Error("не удалось сохранить данные при выходе", "err", err)
	}
	mu.Unlock()

	slog.Info("сервер остановлен. До свидания!")
}
