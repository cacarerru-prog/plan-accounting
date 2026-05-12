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

func TestCalcStats_SalariesFromProfit(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100, Price: 50})
	_ = s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 10, Price: 50, Date: "15.04.2026"})

	// Profit = 500. Сотрудники по умолчанию: 50% + 25% + 25% = 100% → TotalSalaries = 500.
	st := s.CalcStats(4, 2026)
	if st.TotalSalaries != 500 {
		t.Errorf("TotalSalaries: ожидаем 500, получили %v", st.TotalSalaries)
	}
	if st.NetProfit != 0 {
		t.Errorf("NetProfit: ожидаем 0 (после ЗП), получили %v", st.NetProfit)
	}
}
