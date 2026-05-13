// internal/api/api_test.go — общие helpers + тесты внутренних утилит api.
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"plants-app/internal/storage"
)

// newTestServer — поднимает Server с временным Store на tmp-файле.
// Backup'ы отключены, дефолтные сотрудники не очищаются (они нужны для расчёта зарплат).
func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "data.json")
	st := storage.New(tmp)
	st.SetBackupDir("")
	if err := st.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return NewServer(st)
}

// do — выполняет запрос через Routes() и возвращает ответ.
func do(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestIdFromPath(t *testing.T) {
	cases := []struct {
		path, prefix string
		want         int
		ok           bool
	}{
		{"/api/sales/42", "/api/sales/", 42, true},
		{"/api/plants/7/", "/api/plants/", 7, true},
		{"/api/sales/abc", "/api/sales/", 0, false},
		{"/api/sales/", "/api/sales/", 0, false},
	}
	for _, c := range cases {
		got, err := idFromPath(c.path, c.prefix)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("idFromPath(%q): want %d, got %d (err=%v)", c.path, c.want, got, err)
		}
		if !c.ok && err == nil {
			t.Errorf("idFromPath(%q): ожидаем ошибку, получили %d", c.path, got)
		}
	}
}

func TestStaticFiles_NoCacheHeaderForHTML(t *testing.T) {
	srv := newTestServer(t)
	// "/" → static, должен выставить no-cache. Файла нет → 404, но заголовки уже выставлены.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Cache-Control"), "no-cache") {
		t.Errorf("ожидаем no-cache в Cache-Control, получили %q", rec.Header().Get("Cache-Control"))
	}
}

func TestWriteJSON_SetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, map[string]int{"x": 1})
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: ожидаем application/json, получили %q", ct)
	}
}
