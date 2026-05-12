// internal/storage/employees.go — операции над сотрудниками.
package storage

import (
	"errors"

	"plants-app/internal/models"
)

// ErrEmployeeNotFound — сотрудник не найден.
var ErrEmployeeNotFound = errors.New("сотрудник не найден")

// ListEmployees возвращает копию слайса сотрудников.
func (s *Store) ListEmployees() []models.Employee {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Employee, len(s.db.Employees))
	copy(out, s.db.Employees)
	return out
}

// CreateEmployee добавляет сотрудника.
func (s *Store) CreateEmployee(e *models.Employee) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.ID = s.nextID()
	s.db.Employees = append(s.db.Employees, *e)
	return s.saveLocked()
}

// UpdateEmployee заменяет данные сотрудника по ID.
func (s *Store) UpdateEmployee(id int, e *models.Employee) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.db.Employees {
		if existing.ID == id {
			e.ID = id
			s.db.Employees[i] = *e
			return s.saveLocked()
		}
	}
	return ErrEmployeeNotFound
}

// DeleteEmployee удаляет сотрудника.
func (s *Store) DeleteEmployee(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, e := range s.db.Employees {
		if e.ID == id {
			s.db.Employees = append(s.db.Employees[:i], s.db.Employees[i+1:]...)
			return s.saveLocked()
		}
	}
	return ErrEmployeeNotFound
}
