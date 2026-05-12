// internal/storage/plants_test.go — тесты CRUD растений.
package storage

import (
	"errors"
	"testing"

	"plants-app/internal/models"
)

// newTestStore — создаёт временный Store с tmp-файлом для изоляции тестов.
// Backup'ы отключаются, чтобы не засорять рабочую директорию.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir() + "/data.json"
	s := New(tmp)
	s.SetBackupDir("") // отключаем backup'ы в тестах
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestCreatePlant_Basic(t *testing.T) {
	s := newTestStore(t)

	p := &models.Plant{Name: "Туя", Category: "Хвойные", Price: 50, Qty: 10}
	if err := s.CreatePlant(p); err != nil {
		t.Fatalf("CreatePlant: %v", err)
	}
	if p.ID == 0 {
		t.Error("ожидаем ненулевой ID после создания")
	}
	plants := s.ListPlants()
	if len(plants) != 1 {
		t.Fatalf("ожидаем 1 растение, получили %d", len(plants))
	}
	if plants[0].Name != "Туя" {
		t.Errorf("неверное имя: %q", plants[0].Name)
	}
}

func TestCreatePlant_Duplicate(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 5})

	err := s.CreatePlant(&models.Plant{Name: "туя", Qty: 3}) // другой регистр
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("ожидаем ErrDuplicate, получили %v", err)
	}
}

func TestUpdatePlant(t *testing.T) {
	s := newTestStore(t)
	p := &models.Plant{Name: "Туя", Qty: 5, Price: 50}
	_ = s.CreatePlant(p)

	updated := &models.Plant{Name: "Туя западная", Qty: 10, Price: 80}
	if err := s.UpdatePlant(p.ID, updated); err != nil {
		t.Fatalf("UpdatePlant: %v", err)
	}
	plants := s.ListPlants()
	if plants[0].Name != "Туя западная" || plants[0].Qty != 10 {
		t.Errorf("обновление не применилось: %+v", plants[0])
	}
}

func TestUpdatePlant_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdatePlant(999, &models.Plant{Name: "Нет"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидаем ErrNotFound, получили %v", err)
	}
}

func TestDeletePlant(t *testing.T) {
	s := newTestStore(t)
	p := &models.Plant{Name: "Туя", Qty: 5}
	_ = s.CreatePlant(p)

	if err := s.DeletePlant(p.ID); err != nil {
		t.Fatalf("DeletePlant: %v", err)
	}
	if len(s.ListPlants()) != 0 {
		t.Error("растение должно было удалиться")
	}
}

func TestPersistence(t *testing.T) {
	tmp := t.TempDir() + "/data.json"

	// Создаём растение в первом экземпляре.
	s1 := New(tmp)
	s1.SetBackupDir("")
	_ = s1.Load()
	_ = s1.CreatePlant(&models.Plant{Name: "Туя", Qty: 5, Price: 50})

	// Поднимаем второй экземпляр — должен прочитать тот же файл.
	s2 := New(tmp)
	s2.SetBackupDir("")
	if err := s2.Load(); err != nil {
		t.Fatalf("второй Load: %v", err)
	}
	plants := s2.ListPlants()
	if len(plants) != 1 || plants[0].Name != "Туя" {
		t.Errorf("персистентность сломана: %+v", plants)
	}
}
