// internal/storage/stats.go — подсчёт статистики за период.
package storage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MonthNames — названия месяцев на русском для PeriodLabel.
var MonthNames = [13]string{
	"", "Январь", "Февраль", "Март", "Апрель",
	"Май", "Июнь", "Июль", "Август",
	"Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

// TopPlant — позиция в топе растений.
type TopPlant struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
}

// SalaryStat — расчёт зарплаты сотрудника.
type SalaryStat struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
	Amount  float64 `json:"amount"`
}

// MonthlyPoint — данные одного месяца для графика трендов.
type MonthlyPoint struct {
	Month    int     `json:"month"`
	Label    string  `json:"label"`
	Revenue  float64 `json:"revenue"`
	Expenses float64 `json:"expenses"`
	Profit   float64 `json:"profit"`
}

// Stats — итоговая статистика за период (месяц).
type Stats struct {
	PeriodMonth int    `json:"period_month"`
	PeriodYear  int    `json:"period_year"`
	PeriodLabel string `json:"period_label"`

	SalesRevenue    float64 `json:"sales_revenue"`
	ProjectsRevenue float64 `json:"projects_revenue"`
	TotalRevenue    float64 `json:"total_revenue"`
	TotalExpenses   float64 `json:"total_expenses"`
	Profit          float64 `json:"profit"`
	StockValue      float64 `json:"stock_value"`

	ByChannel     map[string]float64 `json:"by_channel"`
	TopPlants     []TopPlant         `json:"top_plants"`
	Salaries      []SalaryStat       `json:"salaries"`
	TotalSalaries float64            `json:"total_salaries"`
	NetProfit     float64            `json:"net_profit"`

	SalesCount    int `json:"sales_count"`
	ProjectsCount int `json:"projects_count"`

	// Предыдущий период — для дельт в KPI-карточках дашборда.
	PrevRevenue       float64 `json:"prev_revenue"`
	PrevExpenses      float64 `json:"prev_expenses"`
	PrevProfit        float64 `json:"prev_profit"`
	PrevSalesCount    int     `json:"prev_sales_count"`
	PrevProjectsCount int     `json:"prev_projects_count"`
}

// CalcStats считает статистику за указанный месяц/год.
// Если month=0 или year=0 — используется текущий месяц.
func (s *Store) CalcStats(month, year int) Stats {
	now := time.Now()
	if month < 1 || month > 12 {
		month = int(now.Month())
	}
	if year < 2020 || year > 2100 {
		year = now.Year()
	}

	snap := s.Snapshot()

	stats := Stats{
		PeriodMonth: month,
		PeriodYear:  year,
		PeriodLabel: fmt.Sprintf("%s %d", MonthNames[month], year),
		ByChannel:   make(map[string]float64),
		TopPlants:   []TopPlant{},
		Salaries:    []SalaryStat{},
	}

	plantTotals := make(map[string]float64)

	for _, sale := range snap.Sales {
		if !matchesPeriod(sale.Date, month, year) {
			continue
		}
		stats.SalesRevenue += sale.Total
		stats.ByChannel[sale.Channel] += sale.Total
		plantTotals[sale.PlantName] += sale.Total
		stats.SalesCount++
	}

	for _, proj := range snap.Projects {
		if !matchesPeriod(proj.Date, month, year) {
			continue
		}
		stats.ProjectsRevenue += proj.Total
		stats.ByChannel[proj.Channel] += proj.Total
		for _, pp := range proj.Plants {
			plantTotals[pp.PlantName] += pp.Total
		}
		stats.ProjectsCount++
	}

	for _, e := range snap.Expenses {
		if !matchesPeriod(e.Date, month, year) {
			continue
		}
		stats.TotalExpenses += e.Amount
	}

	// Стоимость склада — всегда актуальная (не за период).
	for _, p := range snap.Plants {
		stats.StockValue += p.Price * float64(p.Qty)
	}

	stats.TotalRevenue = stats.SalesRevenue + stats.ProjectsRevenue
	stats.Profit = stats.TotalRevenue - stats.TotalExpenses

	for _, emp := range snap.Employees {
		amount := stats.Profit * emp.Percent / 100
		stats.Salaries = append(stats.Salaries, SalaryStat{
			Name: emp.Name, Percent: emp.Percent, Amount: amount,
		})
		stats.TotalSalaries += amount
	}
	stats.NetProfit = stats.Profit - stats.TotalSalaries

	for name, total := range plantTotals {
		stats.TopPlants = append(stats.TopPlants, TopPlant{name, total})
	}
	sort.Slice(stats.TopPlants, func(i, j int) bool {
		return stats.TopPlants[i].Total > stats.TopPlants[j].Total
	})
	if len(stats.TopPlants) > 5 {
		stats.TopPlants = stats.TopPlants[:5]
	}

	// Считаем предыдущий период (предыдущий месяц) для дельт.
	prevMonth, prevYear := month-1, year
	if prevMonth < 1 {
		prevMonth = 12
		prevYear--
	}
	for _, sale := range snap.Sales {
		if matchesPeriod(sale.Date, prevMonth, prevYear) {
			stats.PrevRevenue += sale.Total
			stats.PrevSalesCount++
		}
	}
	for _, proj := range snap.Projects {
		if matchesPeriod(proj.Date, prevMonth, prevYear) {
			stats.PrevRevenue += proj.Total
			stats.PrevProjectsCount++
		}
	}
	for _, e := range snap.Expenses {
		if matchesPeriod(e.Date, prevMonth, prevYear) {
			stats.PrevExpenses += e.Amount
		}
	}
	stats.PrevProfit = stats.PrevRevenue - stats.PrevExpenses

	return stats
}

// CalcMonthlyTrend возвращает данные за 12 месяцев указанного года для графика.
func (s *Store) CalcMonthlyTrend(year int) []MonthlyPoint {
	snap := s.Snapshot()

	points := make([]MonthlyPoint, 12)
	for i := range points {
		points[i] = MonthlyPoint{Month: i + 1, Label: MonthNames[i+1]}
	}

	for _, sale := range snap.Sales {
		m, y := parseDateMonthYear(sale.Date)
		if y == year && m >= 1 && m <= 12 {
			points[m-1].Revenue += sale.Total
		}
	}
	for _, proj := range snap.Projects {
		m, y := parseDateMonthYear(proj.Date)
		if y == year && m >= 1 && m <= 12 {
			points[m-1].Revenue += proj.Total
		}
	}
	for _, e := range snap.Expenses {
		m, y := parseDateMonthYear(e.Date)
		if y == year && m >= 1 && m <= 12 {
			points[m-1].Expenses += e.Amount
		}
	}
	for i := range points {
		points[i].Profit = points[i].Revenue - points[i].Expenses
	}
	return points
}

// matchesPeriod проверяет попадает ли дата dd.mm.yyyy в month/year.
// Кривые даты считаются «вне периода» — не крашим сервер.
func matchesPeriod(date string, month, year int) bool {
	m, y := parseDateMonthYear(date)
	return m == month && y == year
}

// parseDateMonthYear разбирает dd.mm.yyyy и возвращает (month, year).
// При невалидной строке возвращает (0, 0).
func parseDateMonthYear(date string) (month, year int) {
	parts := strings.Split(date, ".")
	if len(parts) != 3 {
		return 0, 0
	}
	m, err1 := strconv.Atoi(parts[1])
	y, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return m, y
}
