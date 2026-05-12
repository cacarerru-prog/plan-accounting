// internal/storage/sales.go — операции над розничными продажами.
package storage

import (
	"errors"
	"fmt"
	"time"

	"plants-app/internal/models"
)

// ErrSaleNotFound — продажа не найдена.
var ErrSaleNotFound = errors.New("продажа не найдена")

// ErrPlantNotInStock — продаваемое растение не найдено на складе (при SkipStock=false).
var ErrPlantNotInStock = errors.New("растение не найдено на складе")

// ErrInsufficientQty — на складе меньше, чем продают.
type ErrInsufficientQty struct {
	Available int
}

func (e *ErrInsufficientQty) Error() string {
	return fmt.Sprintf("на складе только %d шт.", e.Available)
}

// ListSales возвращает последние limit продаж в обратном порядке (новые сверху).
func (s *Store) ListSales(limit int) []models.Sale {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.db.Sales)
	out := make([]models.Sale, n)
	for i, v := range s.db.Sales {
		out[n-1-i] = v
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// CreateSale создаёт продажу. Если !SkipStock — проверяет склад и списывает Qty.
func (s *Store) CreateSale(sale *models.Sale) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !sale.SkipStock {
		idx := s.findPlantIdxLocked(sale.PlantName)
		if idx == -1 {
			return ErrPlantNotInStock
		}
		if sale.Qty > s.db.Plants[idx].Qty {
			return &ErrInsufficientQty{Available: s.db.Plants[idx].Qty}
		}
		s.db.Plants[idx].Qty -= sale.Qty
	}

	sale.ID = s.nextID()
	sale.Total = float64(sale.Qty) * sale.Price
	if sale.Date == "" {
		sale.Date = time.Now().Format("02.01.2006")
	}
	s.db.Sales = append(s.db.Sales, *sale)
	return s.saveLocked()
}

// DeleteSale удаляет продажу. Если !SkipStock — возвращает Qty на склад.
func (s *Store) DeleteSale(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, sale := range s.db.Sales {
		if sale.ID != id {
			continue
		}
		// БАГФИКС: только обычные продажи списывали склад → только их qty возвращаем.
		if !sale.SkipStock {
			if idx := s.findPlantIdxLocked(sale.PlantName); idx != -1 {
				s.db.Plants[idx].Qty += sale.Qty
			}
		}
		s.db.Sales = append(s.db.Sales[:i], s.db.Sales[i+1:]...)
		return s.saveLocked()
	}
	return ErrSaleNotFound
}
