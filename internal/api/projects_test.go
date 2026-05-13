// internal/api/projects_test.go — тесты HTTP-обработчика проектов.
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"plants-app/internal/models"
)

func TestHandleProjects_PostAndList(t *testing.T) {
	srv := newTestServer(t)
	_ = do(t, srv, http.MethodPost, "/api/plants",
		models.Plant{Name: "Туя", Qty: 10, Price: 50})

	rec := do(t, srv, http.MethodPost, "/api/projects", models.Project{
		Client:    "Кафе",
		LaborCost: 200,
		Plants:    []models.ProjectPlant{{PlantName: "Туя", Qty: 2, Price: 50}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: ожидаем 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	rec = do(t, srv, http.MethodGet, "/api/projects", nil)
	var list []models.Project
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 || list[0].Client != "Кафе" {
		t.Errorf("список проектов: %+v", list)
	}
}

func TestHandleProjects_PostValidation(t *testing.T) {
	srv := newTestServer(t)

	// Пустой клиент
	rec := do(t, srv, http.MethodPost, "/api/projects",
		models.Project{Client: "  ", LaborCost: 100})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на пустого клиента, получили %d", rec.Code)
	}

	// Нет растений и нет стоимости работы
	rec = do(t, srv, http.MethodPost, "/api/projects",
		models.Project{Client: "X"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на пустой проект, получили %d", rec.Code)
	}
}

func TestHandleProjects_PostMissingPlant(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, http.MethodPost, "/api/projects", models.Project{
		Client: "Кафе",
		Plants: []models.ProjectPlant{{PlantName: "Нет такого", Qty: 1, Price: 10}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 (растение не найдено), получили %d", rec.Code)
	}
}

func TestHandleProjects_PostInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/api/projects", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400, получили %d", rec.Code)
	}
}

func TestHandleProjects_Delete(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodDelete, "/api/projects/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ожидаем 400 на битый ID, получили %d", rec.Code)
	}

	rec = do(t, srv, http.MethodDelete, "/api/projects/999", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("ожидаем 200 на несуществующий ID, получили %d", rec.Code)
	}
}

func TestHandleProjects_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	rec := do(t, srv, http.MethodPatch, "/api/projects", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидаем 405, получили %d", rec.Code)
	}
}
