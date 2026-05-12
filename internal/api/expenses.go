// internal/api/expenses.go — обработчик /api/expenses[/{id}].
package api

import (
	"errors"
	"net/http"

	"plants-app/internal/models"
	"plants-app/internal/storage"
)

func (s *Server) handleExpenses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.ListExpenses(2000))

	case http.MethodPost:
		var e models.Expense
		if err := readJSON(r, &e); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if e.Amount <= 0 {
			http.Error(w, "Сумма должна быть положительной", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateExpense(&e); err != nil {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, e)

	case http.MethodDelete:
		id, err := idFromPath(r.URL.Path, "/api/expenses/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		if err := s.store.DeleteExpense(id); err != nil && !errors.Is(err, storage.ErrExpenseNotFound) {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	default:
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
	}
}
