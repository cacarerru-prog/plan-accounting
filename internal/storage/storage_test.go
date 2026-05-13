// internal/storage/storage_test.go — тесты сохранения и загрузки данных.
package storage

import (
	"os"
	"testing"

	"plants-app/internal/models"
)

func TestSaveNow(t *testing.T) {
	s := newTestStore(t)

	_ = s.CreateEmployee(&models.Employee{Name: "Александр", Percent: 50})

	if err := s.SaveNow(); err != nil {
		t.Fatalf("SaveNow: %v", err)
	}

	// Файл должен существовать и быть непустым.
	info, err := os.Stat(s.file)
	if err != nil {
		t.Fatalf("файл не найден после SaveNow: %v", err)
	}
	if info.Size() == 0 {
		t.Error("файл данных не должен быть пустым")
	}
}

func TestLoad_PersistsAcrossRestart(t *testing.T) {
	tmp := t.TempDir() + "/data.json"

	// Первый store: создаём растение и сохраняем.
	s1 := New(tmp)
	s1.SetBackupDir("")
	if err := s1.Load(); err != nil {
		t.Fatalf("Load s1: %v", err)
	}
	if err := s1.CreatePlant(&models.Plant{Name: "Туя", Qty: 5, Price: 100}); err != nil {
		t.Fatalf("CreatePlant: %v", err)
	}

	// Второй store: загружаемся из того же файла.
	s2 := New(tmp)
	s2.SetBackupDir("")
	if err := s2.Load(); err != nil {
		t.Fatalf("Load s2: %v", err)
	}

	plants := s2.ListPlants()
	if len(plants) != 1 || plants[0].Name != "Туя" {
		t.Errorf("растения не сохранились между перезапусками: %+v", plants)
	}
}

func TestLoad_DefaultEmployeesWhenEmpty(t *testing.T) {
	// Если в файле нет данных — Load должен подставить дефолтных сотрудников.
	tmp := t.TempDir() + "/data.json"
	s := New(tmp)
	s.SetBackupDir("")
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	list := s.ListEmployees()
	if len(list) != 3 {
		t.Errorf("ожидаем 3 дефолтных сотрудника, получили %d", len(list))
	}
}

func TestLoad_CorruptedFile(t *testing.T) {
	tmp := t.TempDir() + "/data.json"
	// Записываем заведомо невалидный JSON.
	if err := os.WriteFile(tmp, []byte("{ not json"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	s := New(tmp)
	s.SetBackupDir("")
	if err := s.Load(); err == nil {
		t.Error("ожидаем ошибку при битом JSON")
	}
}

func TestNew_DefaultPath(t *testing.T) {
	// Если в New передали пусто — должен использоваться defaultFile.
	s := New("")
	if s.file != defaultFile {
		t.Errorf("ожидаем file=%q, получили %q", defaultFile, s.file)
	}
}
