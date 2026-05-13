// internal/api/sales_test.go — тесты HTTP-обработчика продаж.
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"plants-app/internal/models"
)

func TestHandleSales_PostAndList(t *testing.T) {
	srv := newTestServer(t)
	_ = do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 10, Price: 50})

	rec := do(t, srv, http.MethodPost, "/api/sales",
		models.Sale{PlantName: "Туя", Qty: 2, Price: 50, Channel: "Рынок"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST sale: ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	rec = do(t, srv, http.MethodGet, "/api/sales", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("GET: ожидаем 200, получили %d", rec.Code)
	}
	var list []models.Sale
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 || list[0].PlantName != "Туя" {
		t.Errorf("неверный список: %+v", list)
	}
}

func TestHandleSales_PostValidation(t *testing.T) {
	srv := newTestServer(t)

	cases := []struct {
		name string
		body models.Sale
		want int
	}{
		{"пустое имя", models.Sale{PlantName: "  ", Qty: 1, Price: 10}, http.StatusBadRequest},
		{"qty=0", models.Sale{PlantName: "Туя", Qty: 0, Price: 10}, http.StatusBadRequest},
		{"отрицательная цена", models.Sale{PlantName: "Туя", Qty: 1, Price: -5}, http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := do(t, srv, http.MethodPost, "/api/sales", c.body)
		if rec.Code != c.want {
			t.Errorf("%s: ожидаем %d, получили %d (%s)", c.name, c.want, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleSales_PostPlantNotFound(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/sales",
		models.Sale{PlantName: "Нет такого", Qty: 1, Price: 10})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 (растение не найдено), получили %d", rec.Code)
	}
}

func TestHandleSales_PostInsufficientQty(t *testing.T) {
	srv := newTestServer(t)
	_ = do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 2, Price: 50})
	rec := do(t, srv, http.MethodPost, "/api/sales",
		models.Sale{PlantName: "Туя", Qty: 10, Price: 50})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 (мало на складе), получили %d", rec.Code)
	}
}

func TestHandleSales_Delete(t *testing.T) {
	srv := newTestServer(t)
	_ = do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 5, Price: 50})
	rec := do(t, srv, http.MethodPost, "/api/sales",
		models.Sale{PlantName: "Туя", Qty: 1, Price: 50})
	var sale models.Sale
	_ = json.NewDecoder(rec.Body).Decode(&sale)

	rec = do(t, srv, http.MethodDelete, "/api/sales/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("DELETE с битым ID: ожидаем 400, получили %d", rec.Code)
	}

	rec = do(t, srv, http.MethodDelete, "/api/sales/2", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE: ожидаем 200 (даже если не найден), получили %d", rec.Code)
	}
}

func TestHandleSales_PostInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/sales", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400, получили %d", rec.Code)
	}
}

func TestHandleSales_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPatch, "/api/sales", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}
