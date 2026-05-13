// internal/api/expenses_test.go — тесты HTTP-обработчика расходов.
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"plants-app/internal/models"
)

func TestHandleExpenses_PostAndList(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, http.MethodPost, "/api/expenses",
		models.Expense{Category: "Закупка", Description: "Туя", Amount: 500, Date: "01.04.2026"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var created models.Expense
	_ = json.NewDecoder(rec.Body).Decode(&created)
	if created.ID == 0 {
		t.Error("ID должен быть проставлен")
	}

	rec = do(t, srv, http.MethodGet, "/api/expenses", nil)
	var list []models.Expense
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("ожидаем 1, получили %d", len(list))
	}
}

func TestHandleExpenses_PostValidation(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, http.MethodPost, "/api/expenses",
		models.Expense{Category: "X", Amount: 0})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на amount<=0, получили %d", rec.Code)
	}
}

func TestHandleExpenses_PostInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/expenses", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400, получили %d", rec.Code)
	}
}

func TestHandleExpenses_Delete(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/expenses",
		models.Expense{Category: "X", Amount: 100})
	var created models.Expense
	_ = json.NewDecoder(rec.Body).Decode(&created)

	rec = do(t, srv, http.MethodDelete, "/api/expenses/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("DELETE битый ID: ожидаем 400, получили %d", rec.Code)
	}

	rec = do(t, srv, http.MethodDelete, "/api/expenses/999", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE не найден: ожидаем 200, получили %d", rec.Code)
	}
}

func TestHandleExpenses_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPatch, "/api/expenses", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}
