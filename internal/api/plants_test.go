// internal/api/plants_test.go — тесты HTTP-обработчика растений.
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"plants-app/internal/models"
)

func TestHandlePlants_PostAndList(t *testing.T) {
	srv := newTestServer(t)

	// POST
	rec := do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 10, Price: 50})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var created models.Plant
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == 0 || created.Name != "Туя" {
		t.Errorf("неверный созданный объект: %+v", created)
	}

	// GET
	rec = do(t, srv, http.MethodGet, "/api/plants", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: ожидаем 200, получили %d", rec.Code)
	}
	var list []models.Plant
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("ожидаем 1 элемент, получили %d", len(list))
	}
}

func TestHandlePlants_PostEmptyName(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "   ", Qty: 5})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на пустое имя, получили %d", rec.Code)
	}
}

func TestHandlePlants_PostNegativeFields(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: -5})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на отрицательное qty, получили %d", rec.Code)
	}
}

func TestHandlePlants_PostInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	// Не отправляем тело — body=nil → пустая строка. Должно вернуть 400.
	rec := do(t, srv, http.MethodPost, "/api/plants", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на пустое тело, получили %d", rec.Code)
	}
}

func TestHandlePlants_PostDuplicate(t *testing.T) {
	srv := newTestServer(t)
	_ = do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 5, Price: 50})
	rec := do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "туя", Qty: 3, Price: 60})
	if rec.Code != http.StatusConflict {
		t.Errorf("ожидаем 409 на дубль, получили %d", rec.Code)
	}
}

func TestHandlePlants_PutAndDelete(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 5, Price: 50})
	var created models.Plant
	_ = json.NewDecoder(rec.Body).Decode(&created)

	// PUT
	rec = do(t, srv, http.MethodPut, "/api/plants/1",
		models.Plant{Name: "Туя западная", Qty: 10, Price: 80})
	if rec.Code != http.StatusOK {
		t.Errorf("PUT: ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	// PUT с невалидным ID
	rec = do(t, srv, http.MethodPut, "/api/plants/abc",
		models.Plant{Name: "X"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PUT с битым ID: ожидаем 400, получили %d", rec.Code)
	}

	// PUT несуществующего
	rec = do(t, srv, http.MethodPut, "/api/plants/999",
		models.Plant{Name: "Y"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("PUT с unknown id: ожидаем 404, получили %d", rec.Code)
	}

	// DELETE
	rec = do(t, srv, http.MethodDelete, "/api/plants/1", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE: ожидаем 200, получили %d", rec.Code)
	}

	// DELETE с битым ID
	rec = do(t, srv, http.MethodDelete, "/api/plants/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("DELETE с битым ID: ожидаем 400, получили %d", rec.Code)
	}
}

func TestHandlePlants_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPatch, "/api/plants", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}
