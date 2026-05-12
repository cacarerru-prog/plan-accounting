// internal/api/sales.go — обработчик /api/sales[/{id}].
package api

import (
	"errors"
	"net/http"
	"strings"

	"plants-app/internal/models"
	"plants-app/internal/storage"
)

func (s *Server) handleSales(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.ListSales(2000))

	case http.MethodPost:
		var sale models.Sale
		if err := readJSON(r, &sale); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		sale.PlantName = strings.TrimSpace(sale.PlantName)
		sale.Channel = strings.TrimSpace(sale.Channel)
		if sale.PlantName == "" {
			http.Error(w, "Укажи название", http.StatusBadRequest)
			return
		}
		if sale.Qty <= 0 {
			http.Error(w, "Количество должно быть положительным", http.StatusBadRequest)
			return
		}
		if sale.Price < 0 {
			http.Error(w, "Цена не может быть отрицательной", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateSale(&sale); err != nil {
			switch {
			case errors.Is(err, storage.ErrPlantNotInStock):
				http.Error(w, err.Error(), http.StatusBadRequest)
			default:
				// ErrInsufficientQty — нестатическая, проверяем через тип.
				var insuf *storage.ErrInsufficientQty
				if errors.As(err, &insuf) {
					http.Error(w, insuf.Error(), http.StatusBadRequest)
					return
				}
				http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, sale)

	case http.MethodDelete:
		id, err := idFromPath(r.URL.Path, "/api/sales/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		if err := s.store.DeleteSale(id); err != nil && !errors.Is(err, storage.ErrSaleNotFound) {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	default:
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
	}
}
