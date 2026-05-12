// Package api — HTTP-обработчики.
//
// Каждый обработчик принимает Store через конструктор Server.NewServer
// и реализует один из ресурсов: plants, sales, expenses, projects и т.д.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"plants-app/internal/storage"
)

// Server — HTTP-роутер приложения.
type Server struct {
	store *storage.Store
}

// NewServer — конструктор.
func NewServer(s *storage.Store) *Server {
	return &Server{store: s}
}

// Routes регистрирует все маршруты на стандартном mux и возвращает обработчик.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plants", s.handlePlants)
	mux.HandleFunc("/api/plants/", s.handlePlants)
	mux.HandleFunc("/api/sales", s.handleSales)
	mux.HandleFunc("/api/sales/", s.handleSales)
	mux.HandleFunc("/api/expenses", s.handleExpenses)
	mux.HandleFunc("/api/expenses/", s.handleExpenses)
	mux.HandleFunc("/api/projects", s.handleProjects)
	mux.HandleFunc("/api/projects/", s.handleProjects)
	mux.HandleFunc("/api/employees", s.handleEmployees)
	mux.HandleFunc("/api/employees/", s.handleEmployees)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/stats/monthly", s.handleMonthlyTrend)
	mux.HandleFunc("/api/backups", s.handleBackups)
	mux.HandleFunc("/api/import/csv", s.handleImportCSV)
	mux.Handle("/", staticFiles())
	return logRequests(mux)
}

// staticFiles — отдаёт статику из ./static с no-cache для HTML.
func staticFiles() http.Handler {
	fs := http.FileServer(http.Dir("./static"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		fs.ServeHTTP(w, r)
	})
}

// logRequests — middleware с DEBUG-логом каждого запроса.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("запрос", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// ── helpers ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON failed", "err", err)
	}
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// idFromPath парсит "/api/sales/42" → 42 (нужен префикс для удаления).
func idFromPath(path, prefix string) (int, error) {
	s := strings.TrimPrefix(path, prefix)
	s = strings.Trim(s, "/")
	return strconv.Atoi(s)
}
