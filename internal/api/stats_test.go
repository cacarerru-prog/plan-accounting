// internal/api/stats_test.go — тесты HTTP-обработчиков статистики и импорта CSV.
package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"plants-app/internal/models"
)

func TestHandleStats(t *testing.T) {
	srv := newTestServer(t)
	_ = do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 100, Price: 50})
	_ = do(t, srv, http.MethodPost, "/api/sales",
		models.Sale{PlantName: "Туя", Qty: 2, Price: 50, Date: "15.04.2026"})

	rec := do(t, srv, http.MethodGet, "/api/stats?month=4&year=2026", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: ожидаем 200, получили %d", rec.Code)
	}
	var stats map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats["sales_revenue"] != float64(100) {
		t.Errorf("sales_revenue: ожидаем 100, получили %v", stats["sales_revenue"])
	}
}

func TestHandleStats_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/stats", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}

func TestHandleMonthlyTrend(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, http.MethodGet, "/api/stats/monthly?year=2026", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидаем 200, получили %d", rec.Code)
	}
	var points []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&points)
	if len(points) != 12 {
		t.Errorf("ожидаем 12 точек, получили %d", len(points))
	}
}

func TestHandleMonthlyTrend_DefaultYear(t *testing.T) {
	srv := newTestServer(t)
	// Год вне диапазона → подставится текущий.
	rec := do(t, srv, http.MethodGet, "/api/stats/monthly?year=1900", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("ожидаем 200, получили %d", rec.Code)
	}
}

func TestHandleMonthlyTrend_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/stats/monthly", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}

// importCSVRequest — строит multipart-запрос для /api/import/csv.
func importCSVRequest(t *testing.T, csvBody, category string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if category != "" {
		_ = w.WriteField("category", category)
	}
	fw, err := w.CreateFormFile("file", "price.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte(csvBody)); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/import/csv", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHandleImportCSV_OK(t *testing.T) {
	srv := newTestServer(t)

	csv := "Наименование,Количество,Цена\nТуя,10,50\n"
	req := importCSVRequest(t, csv, "Хвойные")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleImportCSV_BadMultipart(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/import/csv", bytes.NewReader([]byte("not multipart")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=bogus")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на битую форму, получили %d", rec.Code)
	}
}

func TestHandleImportCSV_NoFile(t *testing.T) {
	srv := newTestServer(t)

	// Multipart без поля "file".
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("category", "Хвойные")
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/import/csv", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 (нет файла), получили %d", rec.Code)
	}
}

func TestHandleImportCSV_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/api/import/csv", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}

func TestHandleBackups(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/api/backups", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидаем 200, получили %d", rec.Code)
	}
}

func TestHandleBackups_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/backups", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}
