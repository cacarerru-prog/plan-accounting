// internal/api/plants.go — обработчик /api/plants[/{id}].
package api

import (
	"errors"
	"net/http"
	"strings"

	"plants-app/internal/models"
	"plants-app/internal/storage"
)

func (s *Server) handlePlants(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet:
		writeJSON(w, s.store.ListPlants())

	case r.Method == http.MethodPost:
		var p models.Plant
		if err := readJSON(r, &p); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" {
			http.Error(w, "Название обязательно", http.StatusBadRequest)
			return
		}
		if p.Price < 0 || p.Qty < 0 {
			http.Error(w, "Цена и количество не могут быть отрицательными", http.StatusBadRequest)
			return
		}
		if err := s.store.CreatePlant(&p); err != nil {
			if errors.Is(err, storage.ErrDuplicate) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)

	case r.Method == http.MethodPut:
		id, err := idFromPath(r.URL.Path, "/api/plants/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		var p models.Plant
		if err := readJSON(r, &p); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.store.UpdatePlant(id, &p); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				http.Error(w, "Растение не найдено", http.StatusNotFound)
				return
			}
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	case r.Method == http.MethodDelete:
		id, err := idFromPath(r.URL.Path, "/api/plants/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		if err := s.store.DeletePlant(id); err != nil && !errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	default:
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
	}
}
