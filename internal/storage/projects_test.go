// internal/storage/projects_test.go — тесты проектов озеленения.
package storage

import (
	"errors"
	"strings"
	"testing"

	"plants-app/internal/models"
)

func TestCreateProject_DeductsAllPlants(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10, Price: 50})
	_ = s.CreatePlant(&models.Plant{Name: "Самшит", Qty: 5, Price: 80})

	proj := &models.Project{
		Client:    "Кафе Весна",
		LaborCost: 300,
		Plants: []models.ProjectPlant{
			{PlantName: "Туя", Qty: 3, Price: 50},
			{PlantName: "Самшит", Qty: 2, Price: 80},
		},
	}
	if err := s.CreateProject(proj); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// 3*50 + 2*80 + 300 = 150 + 160 + 300 = 610
	if proj.Total != 610 {
		t.Errorf("Total: ожидаем 610, получили %v", proj.Total)
	}

	plants := s.ListPlants()
	for _, p := range plants {
		if p.Name == "Туя" && p.Qty != 7 {
			t.Errorf("Туя: ожидаем 7, получили %d", p.Qty)
		}
		if p.Name == "Самшит" && p.Qty != 3 {
			t.Errorf("Самшит: ожидаем 3, получили %d", p.Qty)
		}
	}
}

// «Всё или ничего»: если хоть одного растения недостаточно — не списывать ничего.
func TestCreateProject_AtomicCheck(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10})
	_ = s.CreatePlant(&models.Plant{Name: "Самшит", Qty: 1})

	proj := &models.Project{
		Client: "Тест",
		Plants: []models.ProjectPlant{
			{PlantName: "Туя", Qty: 3, Price: 50},
			{PlantName: "Самшит", Qty: 5, Price: 80}, // недостаточно
		},
	}
	err := s.CreateProject(proj)
	if err == nil {
		t.Fatal("ожидаем ошибку — Самшита недостаточно")
	}
	if !strings.Contains(err.Error(), "Самшит") {
		t.Errorf("ожидаем упоминание Самшита в ошибке, получили: %v", err)
	}

	// Туя НЕ должна была списаться, потому что Самшит не прошёл проверку.
	plants := s.ListPlants()
	for _, p := range plants {
		if p.Name == "Туя" && p.Qty != 10 {
			t.Errorf("Туя должна остаться 10, получили %d (атомарность сломана)", p.Qty)
		}
	}
}

func TestDeleteProject_RestoresPlants(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10})

	proj := &models.Project{
		Client: "Тест",
		Plants: []models.ProjectPlant{{PlantName: "Туя", Qty: 4, Price: 50}},
	}
	_ = s.CreateProject(proj)
	if s.ListPlants()[0].Qty != 6 {
		t.Fatalf("предусловие: ожидаем 6 шт. после проекта")
	}

	if err := s.DeleteProject(proj.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if s.ListPlants()[0].Qty != 10 {
		t.Errorf("после удаления проекта Qty не вернулся: %d", s.ListPlants()[0].Qty)
	}
}

func TestDeleteProject_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.DeleteProject(999)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("ожидаем ErrProjectNotFound, получили %v", err)
	}
}
