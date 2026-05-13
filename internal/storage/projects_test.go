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

func TestCreateProject_PlantNotFound(t *testing.T) {
	s := newTestStore(t)
	proj := &models.Project{
		Client: "Тест",
		Plants: []models.ProjectPlant{{PlantName: "Несуществующее", Qty: 1, Price: 10}},
	}
	err := s.CreateProject(proj)
	if err == nil || !strings.Contains(err.Error(), "не найдено") {
		t.Errorf("ожидаем ошибку 'не найдено', получили %v", err)
	}
}

func TestCreateProject_DefaultDate(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10})

	proj := &models.Project{
		Client: "Тест",
		Plants: []models.ProjectPlant{{PlantName: "Туя", Qty: 1, Price: 50}},
	}
	if err := s.CreateProject(proj); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if proj.Date == "" {
		t.Error("Date должен быть автозаполнен")
	}
}

func TestListProjects_ReverseAndLimit(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 100})

	// Три проекта с разными клиентами — в ListProjects должны идти в обратном порядке.
	for _, client := range []string{"A", "B", "C"} {
		_ = s.CreateProject(&models.Project{
			Client: client,
			Plants: []models.ProjectPlant{{PlantName: "Туя", Qty: 1, Price: 10}},
		})
	}

	all := s.ListProjects(0)
	if len(all) != 3 || all[0].Client != "C" || all[2].Client != "A" {
		t.Errorf("ожидаем порядок C,B,A, получили %+v", all)
	}

	limited := s.ListProjects(2)
	if len(limited) != 2 || limited[0].Client != "C" {
		t.Errorf("limit=2: ожидаем C первым, получили %+v", limited)
	}
}

func TestSnapshot_IsIndependentCopy(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10, Price: 50})

	snap := s.Snapshot()
	if len(snap.Plants) != 1 {
		t.Fatalf("ожидаем 1 растение в snapshot, получили %d", len(snap.Plants))
	}

	// Мутация снапшота не должна затронуть store.
	snap.Plants[0].Name = "Mutated"
	if s.ListPlants()[0].Name == "Mutated" {
		t.Error("Snapshot должен быть независимой копией")
	}
}
