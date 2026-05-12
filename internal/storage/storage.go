// Package storage — работа с JSON-файлом data.json и резервными копиями.
//
// Все обращения к данным проходят через Store. Внутри используется sync.RWMutex,
// поэтому Store безопасен для конкурентного доступа из нескольких goroutine.
//
// Атомарная запись: данные сначала пишутся в data.json.tmp, потом os.Rename переименовывает
// файл. Это гарантирует, что data.json никогда не окажется «наполовину записанным».
//
// Резервные копии: каждый час создаётся snapshot в backups/, держится 30 последних.
package storage

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"plants-app/internal/models"
)

const (
	defaultFile        = "data.json"
	defaultBackupDir   = "backups"
	backupKeepCount    = 30
	backupMinIntervalH = 1
)

// Store — потокобезопасное хранилище данных приложения.
type Store struct {
	mu        sync.RWMutex
	db        models.DB
	file      string
	backupDir string
	lastBkp   time.Time
}

// New создаёт Store с указанным файлом данных. Backup'ы пишутся в "backups/" рядом
// с текущей директорией. Файл ещё не читается — для этого вызовите Load.
func New(file string) *Store {
	if file == "" {
		file = defaultFile
	}
	return &Store{file: file, backupDir: defaultBackupDir}
}

// SetBackupDir переопределяет директорию backup'ов (полезно для тестов).
// Пустая строка отключает backup'ы.
func (s *Store) SetBackupDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backupDir = dir
}

// Load читает данные из файла. Если файла нет — создаёт пустую БД
// со стандартным списком сотрудников.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("data.json не найден, создаём новую базу")
			s.db = models.DB{NextID: 1, Employees: models.DefaultEmployees()}
			return nil
		}
		return fmt.Errorf("storage.Load: read: %w", err)
	}
	if err := json.Unmarshal(data, &s.db); err != nil {
		return fmt.Errorf("storage.Load: файл повреждён: %w", err)
	}
	if s.db.NextID == 0 {
		s.db.NextID = 1
	}
	if len(s.db.Employees) == 0 {
		s.db.Employees = models.DefaultEmployees()
	}
	slog.Info("база данных загружена",
		"plants", len(s.db.Plants),
		"sales", len(s.db.Sales),
		"expenses", len(s.db.Expenses),
		"projects", len(s.db.Projects),
	)
	return nil
}

// SaveLocked сохраняет текущий state на диск.
// Вызывается ВНУТРИ методов, которые уже держат mu.Lock() — поэтому без захвата мьютекса.
func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return fmt.Errorf("storage.save: marshal: %w", err)
	}

	// Атомарная запись: пишем во временный файл и переименовываем.
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("storage.save: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.file); err != nil {
		return fmt.Errorf("storage.save: rename: %w", err)
	}

	s.rotateBackup(data)
	return nil
}

// SaveNow форсирует запись (используется при graceful shutdown).
func (s *Store) SaveNow() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// rotateBackup создаёт snapshot не чаще раза в час и подчищает старые.
// Ошибки логируются, но не пробрасываются — backup не должен ронять операцию.
// Если s.backupDir == "" — backup отключён.
func (s *Store) rotateBackup(data []byte) {
	if s.backupDir == "" {
		return
	}
	now := time.Now()
	if !s.lastBkp.IsZero() && now.Sub(s.lastBkp) < backupMinIntervalH*time.Hour {
		return
	}

	if err := os.MkdirAll(s.backupDir, 0755); err != nil {
		slog.Warn("backup: mkdir failed", "err", err)
		return
	}

	name := fmt.Sprintf("%s/data_%s.json", s.backupDir, now.Format("2006-01-02_15-04-05"))
	if err := os.WriteFile(name, data, 0644); err != nil {
		slog.Warn("backup: write failed", "err", err)
		return
	}
	s.lastBkp = now

	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "data_") {
			files = append(files, e.Name())
		}
	}
	if len(files) <= backupKeepCount {
		return
	}
	sort.Strings(files)
	excess := len(files) - backupKeepCount
	for i := 0; i < excess; i++ {
		_ = os.Remove(s.backupDir + "/" + files[i])
	}
}

// nextID — атомарный счётчик ID. Вызывается ВНУТРИ Lock().
func (s *Store) nextID() int {
	id := s.db.NextID
	s.db.NextID++
	return id
}
