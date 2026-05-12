// internal/api/employees.go — обработчик /api/employees[/{id}].
package api

import (
	"errors"
	"net/http"

	"plants-app/internal/models"
	"plants-app/internal/storage"
)

func (s *Server) handleEmployees(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.ListEmployees())

	case http.MethodPost:
		var e models.Employee
		if err := readJSON(r, &e); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if e.Name == "" || e.Percent <= 0 {
			http.Error(w, "Имя и положительный процент обязательны", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateEmployee(&e); err != nil {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, e)

	case http.MethodPut:
		id, err := idFromPath(r.URL.Path, "/api/employees/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		var e models.Employee
		if err := readJSON(r, &e); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.store.UpdateEmployee(id, &e); err != nil {
			if errors.Is(err, storage.ErrEmployeeNotFound) {
				http.Error(w, "Сотрудник не найден", http.StatusNotFound)
				return
			}
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	case http.MethodDelete:
		id, err := idFromPath(r.URL.Path, "/api/employees/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		if err := s.store.DeleteEmployee(id); err != nil && !errors.Is(err, storage.ErrEmployeeNotFound) {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	default:
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
	}
}
