// internal/api/backups.go — обработчик /api/backups.
package api

import "net/http"

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.store.ListBackups())
}
