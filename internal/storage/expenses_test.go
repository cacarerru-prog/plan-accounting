// internal/storage/expenses_test.go — тесты операций над расходами.
package storage

import (
	"testing"

	"plants-app/internal/models"
)

func TestCreateExpense_Basic(t *testing.T) {
	s := newTestStore(t)

	e := &models.Expense{Category: "Аренда", Amount: 500, Date: "10.04.2026"}
	if err := s.CreateExpense(e); err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}
	if e.ID == 0 {
		t.Error("ID должен быть проставлен после создания")
	}
}

func TestCreateExpense_AutoDate(t *testing.T) {
	s := newTestStore(t)

	// Если дата не указана — должна подставиться автоматически.
	e := &models.Expense{Category: "Транспорт", Amount: 100}
	_ = s.CreateExpense(e)

	list := s.ListExpenses(10)
	if list[0].Date == "" {
		t.Error("дата должна быть проставлена автоматически")
	}
}

func TestListExpenses_ReverseOrder(t *testing.T) {
	s := newTestStore(t)

	_ = s.CreateExpense(&models.Expense{Category: "Аренда", Amount: 100, Date: "01.04.2026"})
	_ = s.CreateExpense(&models.Expense{Category: "Реклама", Amount: 200, Date: "02.04.2026"})

	list := s.ListExpenses(10)
	if len(list) != 2 {
		t.Fatalf("ожидаем 2 расхода, получили %d", len(list))
	}
	// ListExpenses возвращает в обратном порядке (последний добавленный — первый).
	if list[0].Amount != 200 {
		t.Errorf("первым должен идти последний добавленный, got Amount=%v", list[0].Amount)
	}
}

func TestListExpenses_LimitApplied(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 5; i++ {
		_ = s.CreateExpense(&models.Expense{Category: "Прочее", Amount: float64(i + 1), Date: "01.04.2026"})
	}

	list := s.ListExpenses(3)
	if len(list) != 3 {
		t.Errorf("limit=3: ожидаем 3, получили %d", len(list))
	}
}

func TestDeleteExpense(t *testing.T) {
	s := newTestStore(t)

	e := &models.Expense{Category: "Закупка", Amount: 300, Date: "05.04.2026"}
	_ = s.CreateExpense(e)

	if err := s.DeleteExpense(e.ID); err != nil {
		t.Fatalf("DeleteExpense: %v", err)
	}
	if len(s.ListExpenses(10)) != 0 {
		t.Error("после удаления список должен быть пуст")
	}
}

func TestDeleteExpense_NotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteExpense(999)
	if err != ErrExpenseNotFound {
		t.Errorf("ожидаем ErrExpenseNotFound, получили %v", err)
	}
}
