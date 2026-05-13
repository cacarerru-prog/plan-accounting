// internal/storage/backups_test.go — тесты резервных копий.
package storage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestListBackups_Empty(t *testing.T) {
	// backupDir пуст (helper отключает backup'ы) → ListBackups возвращает пустой слайс.
	s := newTestStore(t)
	got := s.ListBackups()
	if len(got) != 0 {
		t.Errorf("ожидаем пустой список, получили %+v", got)
	}
}

func TestListBackups_MissingDir(t *testing.T) {
	// backupDir указывает на несуществующую директорию → возвращаем пустой список без ошибки.
	s := New(filepath.Join(t.TempDir(), "data.json"))
	s.SetBackupDir(filepath.Join(t.TempDir(), "no_such_dir"))
	got := s.ListBackups()
	if len(got) != 0 {
		t.Errorf("ожидаем пустой список, получили %+v", got)
	}
}

func TestListBackups_ReadsFiles(t *testing.T) {
	tmp := t.TempDir()
	backupDir := filepath.Join(tmp, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Создаём пару backup-файлов и один лишний (не data_*).
	files := []string{"data_2026-05-01_10-00-00.json", "data_2026-05-02_10-00-00.json", "other.json"}
	for _, f := range files {
		path := filepath.Join(backupDir, f)
		if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	s := New(filepath.Join(tmp, "data.json"))
	s.SetBackupDir(backupDir)

	list := s.ListBackups()
	if len(list) != 2 {
		t.Errorf("ожидаем 2 backup-файла (без other.json), получили %d: %+v", len(list), list)
	}
	for _, b := range list {
		if b.Name == "other.json" {
			t.Error("other.json не должен попасть в список")
		}
	}
}

func TestRotateBackup_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	backupDir := filepath.Join(tmp, "backups")

	s := New(filepath.Join(tmp, "data.json"))
	s.SetBackupDir(backupDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// SaveNow → внутри saveLocked → rotateBackup создаст файл (lastBkp пуст).
	if err := s.SaveNow(); err != nil {
		t.Fatalf("SaveNow: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("ожидаем хотя бы один backup-файл")
	}
}

func TestRotateBackup_RespectsMinInterval(t *testing.T) {
	tmp := t.TempDir()
	backupDir := filepath.Join(tmp, "backups")

	s := New(filepath.Join(tmp, "data.json"))
	s.SetBackupDir(backupDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Первый Save → backup создаётся.
	if err := s.SaveNow(); err != nil {
		t.Fatalf("SaveNow 1: %v", err)
	}
	first, _ := os.ReadDir(backupDir)
	// Второй Save сразу → backup не должен создаться (lastBkp недавно).
	if err := s.SaveNow(); err != nil {
		t.Fatalf("SaveNow 2: %v", err)
	}
	second, _ := os.ReadDir(backupDir)

	if len(first) != len(second) {
		t.Errorf("повторный Save в течение часа не должен создавать новый backup: было %d, стало %d",
			len(first), len(second))
	}
}

func TestRotateBackup_KeepsLimit(t *testing.T) {
	tmp := t.TempDir()
	backupDir := filepath.Join(tmp, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Заполняем директорию старыми backup-файлами (больше лимита).
	for i := 0; i < backupKeepCount+5; i++ {
		name := filepath.Join(backupDir,
			"data_2025-01-01_00-00-"+padZero(i)+".json")
		if err := os.WriteFile(name, []byte(`{}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	s := New(filepath.Join(tmp, "data.json"))
	s.SetBackupDir(backupDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// SaveNow → создаст новый файл и подчистит лишние.
	if err := s.SaveNow(); err != nil {
		t.Fatalf("SaveNow: %v", err)
	}

	entries, _ := os.ReadDir(backupDir)
	if len(entries) > backupKeepCount {
		t.Errorf("ожидаем не более %d файлов, осталось %d", backupKeepCount, len(entries))
	}
}

// padZero — простой паддинг до 2 цифр для имён файлов в тесте.
func padZero(i int) string {
	if i < 10 {
		return "0" + strconv.Itoa(i)
	}
	return strconv.Itoa(i)
}
