// internal/api/employees_test.go — тесты HTTP-обработчика сотрудников.
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"plants-app/internal/models"
)

func TestHandleEmployees_GetReturnsDefault(t *testing.T) {
	srv := newTestServer(t)

	// Load() уже подставил 3 дефолтных — должны прийти через GET.
	rec := do(t, srv, http.MethodGet, "/api/employees", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: ожидаем 200, получили %d", rec.Code)
	}
	var list []models.Employee
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 3 {
		t.Errorf("ожидаем 3 дефолтных, получили %d", len(list))
	}
}

func TestHandleEmployees_PostValidation(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, http.MethodPost, "/api/employees",
		models.Employee{Name: "", Percent: 50})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на пустое имя, получили %d", rec.Code)
	}

	rec = do(t, srv, http.MethodPost, "/api/employees",
		models.Employee{Name: "X", Percent: 0})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на percent=0, получили %d", rec.Code)
	}
}

func TestHandleEmployees_PostInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/employees", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400, получили %d", rec.Code)
	}
}

func TestHandleEmployees_PutAndDelete(t *testing.T) {
	srv := newTestServer(t)

	// PUT существующего (ID=1 — Елена)
	rec := do(t, srv, http.MethodPut, "/api/employees/1",
		models.Employee{Name: "Елена К.", Percent: 60})
	if rec.Code != http.StatusOK {
		t.Errorf("PUT: ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	// PUT с битым ID
	rec = do(t, srv, http.MethodPut, "/api/employees/abc",
		models.Employee{Name: "X", Percent: 10})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PUT битый ID: ожидаем 400, получили %d", rec.Code)
	}

	// PUT несуществующего
	rec = do(t, srv, http.MethodPut, "/api/employees/999",
		models.Employee{Name: "X", Percent: 10})
	if rec.Code != http.StatusNotFound {
		t.Errorf("PUT не найден: ожидаем 404, получили %d", rec.Code)
	}

	// PUT с битым JSON
	rec = do(t, srv, http.MethodPut, "/api/employees/1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PUT битый JSON: ожидаем 400, получили %d", rec.Code)
	}

	// DELETE существующего
	rec = do(t, srv, http.MethodDelete, "/api/employees/1", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE: ожидаем 200, получили %d", rec.Code)
	}

	// DELETE с битым ID
	rec = do(t, srv, http.MethodDelete, "/api/employees/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("DELETE битый ID: ожидаем 400, получили %d", rec.Code)
	}
}

func TestHandleEmployees_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPatch, "/api/employees", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}
