// internal/storage/backups.go — список резервных копий.
package storage

import (
	"os"
	"strings"
	"time"
)

// BackupFile — информация об одном файле резервной копии.
type BackupFile struct {
	Name    string `json:"name"`
	SizeKB  int64  `json:"size_kb"`
	ModTime string `json:"mod_time"`
}

// ListBackups возвращает список файлов резервных копий (новые сначала).
func (s *Store) ListBackups() []BackupFile {
	s.mu.RLock()
	dir := s.backupDir
	s.mu.RUnlock()

	if dir == "" {
		return []BackupFile{}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []BackupFile{}
	}

	result := make([]BackupFile, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.IsDir() || !strings.HasPrefix(e.Name(), "data_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, BackupFile{
			Name:    e.Name(),
			SizeKB:  info.Size() / 1024,
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	return result
}
