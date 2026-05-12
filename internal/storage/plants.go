// internal/storage/plants.go — операции над растениями (склад).
package storage

import (
	"errors"
	"strings"

	"plants-app/internal/models"
)

// ErrNotFound — растение не найдено.
var ErrNotFound = errors.New("растение не найдено")

// ErrDuplicate — растение с таким именем уже есть.
var ErrDuplicate = errors.New("растение с таким названием уже есть на складе")

// ListPlants возвращает копию слайса растений (безопасно для модификации читателем).
func (s *Store) ListPlants() []models.Plant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Plant, len(s.db.Plants))
	copy(out, s.db.Plants)
	return out
}

// CreatePlant добавляет растение. Возвращает ErrDuplicate, если имя уже используется.
func (s *Store) CreatePlant(p *models.Plant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.db.Plants {
		if strings.EqualFold(existing.Name, p.Name) {
			return ErrDuplicate
		}
	}
	p.ID = s.nextID()
	s.db.Plants = append(s.db.Plants, *p)
	return s.saveLocked()
}

// UpdatePlant полностью заменяет растение по ID (поле ID игнорируется в payload, берётся из аргумента).
func (s *Store) UpdatePlant(id int, p *models.Plant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.db.Plants {
		if existing.ID == id {
			p.ID = id
			s.db.Plants[i] = *p
			return s.saveLocked()
		}
	}
	return ErrNotFound
}

// DeletePlant удаляет растение по ID.
func (s *Store) DeletePlant(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.db.Plants {
		if p.ID == id {
			s.db.Plants = append(s.db.Plants[:i], s.db.Plants[i+1:]...)
			return s.saveLocked()
		}
	}
	return ErrNotFound
}

// FindPlantByName возвращает индекс растения в слайсе (под Lock) или -1.
// Используется внутренне в Sale/Project операциях.
func (s *Store) findPlantIdxLocked(name string) int {
	for i := range s.db.Plants {
		if strings.EqualFold(s.db.Plants[i].Name, name) {
			return i
		}
	}
	return -1
}
