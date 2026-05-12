// internal/storage/expenses.go — операции над расходами.
package storage

import (
	"errors"
	"time"

	"plants-app/internal/models"
)

// ErrExpenseNotFound — расход не найден.
var ErrExpenseNotFound = errors.New("расход не найден")

// ListExpenses возвращает последние limit расходов в обратном порядке.
func (s *Store) ListExpenses(limit int) []models.Expense {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.db.Expenses)
	out := make([]models.Expense, n)
	for i, v := range s.db.Expenses {
		out[n-1-i] = v
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// CreateExpense добавляет расход.
func (s *Store) CreateExpense(e *models.Expense) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.ID = s.nextID()
	if e.Date == "" {
		e.Date = time.Now().Format("02.01.2006")
	}
	s.db.Expenses = append(s.db.Expenses, *e)
	return s.saveLocked()
}

// DeleteExpense удаляет расход по ID.
func (s *Store) DeleteExpense(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, e := range s.db.Expenses {
		if e.ID == id {
			s.db.Expenses = append(s.db.Expenses[:i], s.db.Expenses[i+1:]...)
			return s.saveLocked()
		}
	}
	return ErrExpenseNotFound
}
