// internal/storage/stats_test.go — тесты статистики.
package storage

import (
	"testing"

	"plants-app/internal/models"
)

func TestCalcStats_Basic(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})

	// Продажа в апреле 2026.
	_ = s.CreateSale(&models.Sale{
		PlantName: "Туя", Qty: 2, Price: 50, Channel: "Рынок", Date: "15.04.2026",
	})
	// Расход в апреле 2026.
	_ = s.CreateExpense(&models.Expense{Category: "Закупка", Amount: 30, Date: "10.04.2026"})

	st := s.CalcStats(4, 2026)
	if st.SalesRevenue != 100 {
		t.Errorf("SalesRevenue: ожидаем 100, получили %v", st.SalesRevenue)
	}
	if st.TotalExpenses != 30 {
		t.Errorf("TotalExpenses: ожидаем 30, получили %v", st.TotalExpenses)
	}
	if st.Profit != 70 {
		t.Errorf("Profit: ожидаем 70, получили %v", st.Profit)
	}
	if st.PeriodLabel != "Апрель 2026" {
		t.Errorf("PeriodLabel: ожидаем 'Апрель 2026', получили %q", st.PeriodLabel)
	}
}

func TestCalcStats_FiltersByPeriod(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})

	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 1, Price: 50, Date: "15.03.2026"})
	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 2, Price: 50, Date: "15.04.2026"})

	// За апрель попадает только вторая продажа.
	st := s.CalcStats(4, 2026)
	if st.SalesCount != 1 || st.SalesRevenue != 100 {
		t.Errorf("апрель: ожидаем 1 продажу на 100, получили %d на %v",
			st.SalesCount, st.SalesRevenue)
	}
}

func TestCalcStats_BadDateSkipped(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})

	// Кривая дата → запись не должна попасть в статистику и не должна крашить сервис.
	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 1, Price: 50, Date: "abc"})

	st := s.CalcStats(4, 2026)
	if st.SalesCount != 0 {
		t.Errorf("кривая дата должна была быть отфильтрована: count=%d", st.SalesCount)
	}
}

func TestCalcStats_TopPlants(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})
	_ = s.CreatePlant(&models.Plant{Name: "Самшит", Qty: 100, Price: 80})

	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 1, Price: 50, Date: "15.04.2026"})
	_ = s.CreateSale(&models.Sale{PlantName: "Самшит", Qty: 3, Price: 80, Date: "15.04.2026"})

	st := s.CalcStats(4, 2026)
	if len(st.TopPlants) != 2 || st.TopPlants[0].Name != "Самшит" {
		t.Errorf("ожидаем Самшит на 1-м месте, получили %+v", st.TopPlants)
	}
}

func TestCalcMonthlyTrend(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})

	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 2, Price: 50, Date: "15.04.2026"})
	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 1, Price: 50, Date: "10.05.2026"})
	_ = s.CreateExpense(&models.Expense{Category: "Закупка", Amount: 30, Date: "20.04.2026"})
	// Расход 2025 года не должен попасть в тренд 2026.
	_ = s.CreateExpense(&models.Expense{Category: "Закупка", Amount: 999, Date: "15.04.2025"})

	points := s.CalcMonthlyTrend(2026)
	if len(points) != 12 {
		t.Fatalf("ожидаем 12 точек, получили %d", len(points))
	}
	// Апрель = index 3
	if points[3].Revenue != 100 || points[3].Expenses != 30 || points[3].Profit != 70 {
		t.Errorf("апрель 2026: Revenue=100/Expenses=30/Profit=70, получили %+v", points[3])
	}
	// Май = index 4
	if points[4].Revenue != 50 {
		t.Errorf("май 2026 Revenue: ожидаем 50, получили %v", points[4].Revenue)
	}
	// Январь не должен содержать данных 2025 года.
	if points[0].Expenses != 0 {
		t.Errorf("январь 2026: расходы должны быть 0, получили %v", points[0].Expenses)
	}
}

func TestCalcMonthlyTrend_IncludesProjects(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})

	_ = s.CreateProject(&models.Project{
		Client:    "Кафе",
		Date:      "10.04.2026",
		LaborCost: 200,
		Plants:    []models.ProjectPlant{{PlantName: "Туя", Qty: 1, Price: 50}},
	})

	points := s.CalcMonthlyTrend(2026)
	// 50 + 200 = 250
	if points[3].Revenue != 250 {
		t.Errorf("апрель 2026 с проектом: Revenue=250, получили %v", points[3].Revenue)
	}
}

func TestCalcStats_SalariesFromProfit(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})
	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 10, Price: 50, Date: "15.04.2026"})
	// Восстанавливаем сотрудников (helper их вычистил для изоляции).
	_ = s.CreateEmployee(&models.Employee{Name: "Елена", Percent: 50})
	_ = s.CreateEmployee(&models.Employee{Name: "Александр", Percent: 25})
	_ = s.CreateEmployee(&models.Employee{Name: "Данила", Percent: 25})

	// Profit = 500. Сотрудники: 50% + 25% + 25% = 100% → TotalSalaries = 500.
	st := s.CalcStats(4, 2026)
	if st.TotalSalaries != 500 {
		t.Errorf("TotalSalaries: ожидаем 500, получили %v", st.TotalSalaries)
	}
	if st.NetProfit != 0 {
		t.Errorf("NetProfit: ожидаем 0 (после ЗП), получили %v", st.NetProfit)
	}
}
