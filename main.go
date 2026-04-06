package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

var db DB

func loadDB() {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		db = DB{
			NextID: 1,
			Employees: []Employee{
				{ID: 1, Name: "Елена", Percent: 50},
				{ID: 2, Name: "Александр", Percent: 25},
				{ID: 3, Name: "Данила", Percent: 25},
			},
		}
		return
	}
	json.Unmarshal(data, &db)
	if db.NextID == 0 {
		db.NextID = 1
	}
	if len(db.Employees) == 0 {
		db.Employees = []Employee{
			{ID: 1, Name: "Елена", Percent: 50},
			{ID: 2, Name: "Александр", Percent: 25},
			{ID: 3, Name: "Данила", Percent: 25},
		}
	}
}

func saveDB() {
	data, _ := json.MarshalIndent(db, "", "  ")
	os.WriteFile(dataFile, data, 0644)
}

func nextID() int {
	id := db.NextID
	db.NextID++
	return id
}

// ─── HELPERS ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func idFromPath(path, prefix string) int {
	s := strings.TrimPrefix(path, prefix)
	s = strings.Trim(s, "/")
	id, _ := strconv.Atoi(s)
	return id
}

func today() string {
	return time.Now().Format("02.01.2006")
}

// ─── ОБРАБОТЧИКИ: РАСТЕНИЯ ────────────────────────────────

func handlePlants(w http.ResponseWriter, r *http.Request) {
	// GET /api/plants
	if r.Method == http.MethodGet && r.URL.Path == "/api/plants" {
		if db.Plants == nil {
			writeJSON(w, []Plant{})
		} else {
			writeJSON(w, db.Plants)
		}
		return
	}

	// POST /api/plants
	if r.Method == http.MethodPost {
		var p Plant
		if err := readJSON(r, &p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p.ID = nextID()
		db.Plants = append(db.Plants, p)
		saveDB()
		writeJSON(w, p)
		return
	}

	// DELETE /api/plants/{id}
	if r.Method == http.MethodDelete {
		id := idFromPath(r.URL.Path, "/api/plants/")
		for i, p := range db.Plants {
			if p.ID == id {
				db.Plants = append(db.Plants[:i], db.Plants[i+1:]...)
				break
			}
		}
		saveDB()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}

	// PUT /api/plants/{id}
	if r.Method == http.MethodPut {
		id := idFromPath(r.URL.Path, "/api/plants/")
		var p Plant
		readJSON(r, &p)
		for i, pl := range db.Plants {
			if pl.ID == id {
				p.ID = id
				db.Plants[i] = p
				break
			}
		}
		saveDB()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ОБРАБОТЧИКИ: ПРОДАЖИ ─────────────────────────────────

func handleSales(w http.ResponseWriter, r *http.Request) {
	// GET
	if r.Method == http.MethodGet && r.URL.Path == "/api/sales" {
		if db.Sales == nil {
			writeJSON(w, []Sale{})
		} else {
			// последние 100 в обратном порядке
			s := db.Sales
			result := make([]Sale, len(s))
			for i, v := range s {
				result[len(s)-1-i] = v
			}
			if len(result) > 100 {
				result = result[:100]
			}
			writeJSON(w, result)
		}
		return
	}

	// POST
	if r.Method == http.MethodPost {
		var s Sale
		readJSON(r, &s)
		s.ID = nextID()
		s.Total = float64(s.Qty) * s.Price
		if s.Date == "" {
			s.Date = today()
		}
		db.Sales = append(db.Sales, s)

		// списываем остаток
		for i, p := range db.Plants {
			if strings.EqualFold(p.Name, s.PlantName) {
				db.Plants[i].Qty -= s.Qty
				if db.Plants[i].Qty < 0 {
					db.Plants[i].Qty = 0
				}
				break
			}
		}

		saveDB()
		writeJSON(w, s)
		return
	}

	// DELETE
	if r.Method == http.MethodDelete {
		id := idFromPath(r.URL.Path, "/api/sales/")
		for i, s := range db.Sales {
			if s.ID == id {
				db.Sales = append(db.Sales[:i], db.Sales[i+1:]...)
				break
			}
		}
		saveDB()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ОБРАБОТЧИКИ: РАСХОДЫ ─────────────────────────────────

func handleExpenses(w http.ResponseWriter, r *http.Request) {
	// GET
	if r.Method == http.MethodGet && r.URL.Path == "/api/expenses" {
		if db.Expenses == nil {
			writeJSON(w, []Expense{})
		} else {
			e := db.Expenses
			result := make([]Expense, len(e))
			for i, v := range e {
				result[len(e)-1-i] = v
			}
			if len(result) > 100 {
				result = result[:100]
			}
			writeJSON(w, result)
		}
		return
	}

	// POST
	if r.Method == http.MethodPost {
		var e Expense
		readJSON(r, &e)
		e.ID = nextID()
		if e.Date == "" {
			e.Date = today()
		}
		db.Expenses = append(db.Expenses, e)
		saveDB()
		writeJSON(w, e)
		return
	}

	// DELETE
	if r.Method == http.MethodDelete {
		id := idFromPath(r.URL.Path, "/api/expenses/")
		for i, e := range db.Expenses {
			if e.ID == id {
				db.Expenses = append(db.Expenses[:i], db.Expenses[i+1:]...)
				break
			}
		}
		saveDB()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── ИМПОРТ CSV ───────────────────────────────────────────

func handleImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	r.ParseMultipartForm(10 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file", 400)
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

	imported := 0
	skipped := 0
	lineNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		lineNum++

		// пропускаем заголовки, пустые строки, ИТОГО
		if len(record) < 4 {
			continue
		}
		name := strings.TrimSpace(record[1])
		if name == "" || name == "Наименование" || strings.HasPrefix(name, "ИТОГО") {
			continue
		}
		// пропускаем строку-заголовок раздела (первая строка)
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

		// проверяем — нет ли уже такого растения
		found := false
		for i, p := range db.Plants {
			if strings.EqualFold(p.Name, name) {
				// обновляем цену и количество
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

	saveDB()
	writeJSON(w, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"total":    len(db.Plants),
	})
}

// ─── СОТРУДНИКИ И ЗАРПЛАТЫ ────────────────────────────────

func handleEmployees(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, db.Employees)
		return
	}
	if r.Method == http.MethodPost {
		var e Employee
		readJSON(r, &e)
		e.ID = nextID()
		db.Employees = append(db.Employees, e)
		saveDB()
		writeJSON(w, e)
		return
	}
	if r.Method == http.MethodPut {
		id := idFromPath(r.URL.Path, "/api/employees/")
		var e Employee
		readJSON(r, &e)
		for i, emp := range db.Employees {
			if emp.ID == id {
				e.ID = id
				db.Employees[i] = e
				break
			}
		}
		saveDB()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method == http.MethodDelete {
		id := idFromPath(r.URL.Path, "/api/employees/")
		for i, e := range db.Employees {
			if e.ID == id {
				db.Employees = append(db.Employees[:i], db.Employees[i+1:]...)
				break
			}
		}
		saveDB()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
}

// ─── СТАТИСТИКА (расширенная) ─────────────────────────────

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
	stats := Stats{
		ByChannel: make(map[string]float64),
		TopPlants: []TopPlant{},
		Salaries:  []SalaryStat{},
	}

	for _, s := range db.Sales {
		stats.TotalRevenue += s.Total
		stats.ByChannel[s.Channel] += s.Total
	}
	for _, e := range db.Expenses {
		stats.TotalExpenses += e.Amount
	}
	for _, p := range db.Plants {
		stats.StockValue += p.Price * float64(p.Qty)
	}
	stats.Profit = stats.TotalRevenue - stats.TotalExpenses

	// зарплаты от чистой прибыли
	for _, emp := range db.Employees {
		amount := stats.Profit * emp.Percent / 100
		stats.Salaries = append(stats.Salaries, SalaryStat{
			Name:    emp.Name,
			Percent: emp.Percent,
			Amount:  amount,
		})
		stats.TotalSalaries += amount
	}
	stats.NetProfit = stats.Profit - stats.TotalSalaries

	// топ растений
	plantTotals := make(map[string]float64)
	for _, s := range db.Sales {
		plantTotals[s.PlantName] += s.Total
	}
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
	loadDB()

	http.HandleFunc("/", router)

	fmt.Println("====================================")
	fmt.Println("  Растения — учёт запущен!")
	fmt.Println("  Открой браузер: http://localhost:8080")
	fmt.Println("  Данные сохраняются в: data.json")
	fmt.Println("  Для остановки: Ctrl+C")
	fmt.Println("====================================")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
