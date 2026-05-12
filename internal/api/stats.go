// internal/api/stats.go — обработчики /api/stats, /api/stats/monthly и /api/import/csv.
package api

import (
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	writeJSON(w, s.store.CalcStats(month, year))
}

func (s *Server) handleMonthlyTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year < 2020 || year > 2100 {
		year = time.Now().Year()
	}
	writeJSON(w, s.store.CalcMonthlyTrend(year))
}

func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Ошибка парсинга формы: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Файл не найден", http.StatusBadRequest)
		return
	}
	defer file.Close()

	category := r.FormValue("category")

	res, err := s.store.ImportCSV(file, category)
	if err != nil {
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}
