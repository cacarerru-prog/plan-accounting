// internal/storage/projects.go — операции над проектами озеленения.
package storage

import (
	"errors"
	"fmt"
	"time"

	"plants-app/internal/models"
)

// ErrProjectNotFound — проект не найден.
var ErrProjectNotFound = errors.New("проект не найден")

// ListProjects возвращает последние limit проектов в обратном порядке.
func (s *Store) ListProjects(limit int) []models.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.db.Projects)
	out := make([]models.Project, n)
	for i, v := range s.db.Projects {
		out[n-1-i] = v
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// CreateProject создаёт проект: проверяет наличие всех растений, списывает их со склада
// и считает итоговую сумму (растения + работа).
func (s *Store) CreateProject(p *models.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Шаг 1: всё или ничего — проверяем наличие всех растений ДО списания.
	for _, pp := range p.Plants {
		idx := s.findPlantIdxLocked(pp.PlantName)
		if idx == -1 {
			return fmt.Errorf("растение %q не найдено на складе", pp.PlantName)
		}
		if pp.Qty > s.db.Plants[idx].Qty {
			return fmt.Errorf("растение %q: на складе только %d шт., запрошено %d",
				pp.PlantName, s.db.Plants[idx].Qty, pp.Qty)
		}
	}

	// Шаг 2: списываем со склада и считаем итог.
	p.Total = p.LaborCost
	for i := range p.Plants {
		p.Plants[i].Total = float64(p.Plants[i].Qty) * p.Plants[i].Price
		p.Total += p.Plants[i].Total
		if idx := s.findPlantIdxLocked(p.Plants[i].PlantName); idx != -1 {
			s.db.Plants[idx].Qty -= p.Plants[i].Qty
		}
	}

	p.ID = s.nextID()
	if p.Date == "" {
		p.Date = time.Now().Format("02.01.2006")
	}
	s.db.Projects = append(s.db.Projects, *p)
	return s.saveLocked()
}

// DeleteProject удаляет проект и возвращает все его растения на склад.
func (s *Store) DeleteProject(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, proj := range s.db.Projects {
		if proj.ID != id {
			continue
		}
		// Возвращаем все растения обратно на склад.
		for _, pp := range proj.Plants {
			if idx := s.findPlantIdxLocked(pp.PlantName); idx != -1 {
				s.db.Plants[idx].Qty += pp.Qty
			}
		}
		s.db.Projects = append(s.db.Projects[:i], s.db.Projects[i+1:]...)
		return s.saveLocked()
	}
	return ErrProjectNotFound
}

// snapshotForStats — копии всех слайсов, чтобы стат-функция считала без блокировки.
type Snapshot struct {
	Plants    []models.Plant
	Sales     []models.Sale
	Expenses  []models.Expense
	Employees []models.Employee
	Projects  []models.Project
}

// Snapshot возвращает копию всех данных под RLock — для подсчёта статистики.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := Snapshot{
		Plants:    make([]models.Plant, len(s.db.Plants)),
		Sales:     make([]models.Sale, len(s.db.Sales)),
		Expenses:  make([]models.Expense, len(s.db.Expenses)),
		Employees: make([]models.Employee, len(s.db.Employees)),
		Projects:  make([]models.Project, len(s.db.Projects)),
	}
	copy(snap.Plants, s.db.Plants)
	copy(snap.Sales, s.db.Sales)
	copy(snap.Expenses, s.db.Expenses)
	copy(snap.Employees, s.db.Employees)
	copy(snap.Projects, s.db.Projects)
	return snap
}
