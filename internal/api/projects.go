// internal/api/projects.go — обработчик /api/projects[/{id}].
package api

import (
	"errors"
	"net/http"
	"strings"

	"plants-app/internal/models"
	"plants-app/internal/storage"
)

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.ListProjects(50))

	case http.MethodPost:
		var p models.Project
		if err := readJSON(r, &p); err != nil {
			http.Error(w, "Невалидный JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		p.Client = strings.TrimSpace(p.Client)
		if p.Client == "" {
			http.Error(w, "Укажи название заведения (client)", http.StatusBadRequest)
			return
		}
		if len(p.Plants) == 0 && p.LaborCost == 0 {
			http.Error(w, "Проект должен содержать растения или стоимость работы", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateProject(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, p)

	case http.MethodDelete:
		id, err := idFromPath(r.URL.Path, "/api/projects/")
		if err != nil {
			http.Error(w, "Невалидный ID", http.StatusBadRequest)
			return
		}
		if err := s.store.DeleteProject(id); err != nil && !errors.Is(err, storage.ErrProjectNotFound) {
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	default:
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
	}
}
