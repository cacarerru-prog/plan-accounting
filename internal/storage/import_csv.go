// internal/storage/import_csv.go — импорт прайс-листа из CSV.
//
// Понимает русский формат Google Sheets:
//   - десятичная запятая ("25,50")
//   - неразрывный пробел в разрядах ("1 000")
//   - "—" в количестве (оставляет старое)
//   - произвольные заголовки до строки с "Наименование"
package storage

import (
	"encoding/csv"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"plants-app/internal/models"
)

// ImportResult — итог импорта одного файла.
type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Total    int `json:"total"`
}

// ImportCSV читает CSV из reader и обновляет или добавляет растения.
// category используется как дефолтная категория для новых растений.
func (s *Store) ImportCSV(reader io.Reader, category string) (*ImportResult, error) {
	if category == "" {
		category = "Лиственные"
	}

	r := csv.NewReader(reader)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	// Индексы колонок: -1 = не найдены.
	nameIdx, qtyIdx, priceIdx, sizeIdx := -1, -1, -1, -1
	headerParsed := false

	detectHeader := func(record []string) {
		for i, raw := range record {
			cell := strings.ToLower(cleanCell(raw))
			switch {
			case strings.Contains(cell, "наименование") || strings.Contains(cell, "название"):
				nameIdx = i
			case strings.Contains(cell, "наличие") || strings.Contains(cell, "кол-во") ||
				strings.Contains(cell, "количество") || strings.Contains(cell, "шт"):
				qtyIdx = i
			case strings.Contains(cell, "цена") || strings.Contains(cell, "стоимость"):
				priceIdx = i
			case strings.Contains(cell, "размер") || strings.Contains(cell, "контейнер"):
				sizeIdx = i
			}
		}
		headerParsed = nameIdx >= 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res := &ImportResult{}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("CSV: пропускаем строку с ошибкой", "err", err)
			continue
		}
		if len(record) == 0 {
			continue
		}
		record[0] = cleanCell(record[0])

		if isHeaderOrTitle(record) {
			detectHeader(record)
			continue
		}
		if !headerParsed {
			detectHeader(record)
			if headerParsed {
				continue
			}
		}

		// Если колонка имени не найдена — пытаемся угадать по структуре:
		// если первая ячейка — число (номер), значит имя в колонке 1.
		if nameIdx < 0 {
			if len(record) >= 2 {
				if _, err := strconv.Atoi(strings.TrimSpace(record[0])); err == nil {
					nameIdx = 1
				} else {
					nameIdx = 0
				}
			}
		}
		if nameIdx < 0 || nameIdx >= len(record) {
			res.Skipped++
			continue
		}

		name := cleanCell(record[nameIdx])
		nameLower := strings.ToLower(name)
		if name == "" || strings.HasPrefix(nameLower, "итого") || strings.HasPrefix(nameLower, "всего") {
			continue
		}

		// Количество.
		newQty := -1
		if qtyIdx >= 0 && qtyIdx < len(record) {
			if q, err := parseNum(record[qtyIdx]); err == nil && q >= 0 {
				newQty = int(q)
			}
		} else if qtyIdx < 0 {
			// Если колонки явно нет — пытаемся взять число из последней колонки.
			for col := len(record) - 1; col > nameIdx; col-- {
				if q, err := parseNum(record[col]); err == nil && q >= 0 && q < 100000 {
					if priceIdx < 0 {
						newQty = int(q)
					}
					break
				}
			}
		}

		// Цена.
		newPrice := -1.0
		if priceIdx >= 0 && priceIdx < len(record) {
			if p, err := parseNum(record[priceIdx]); err == nil && p > 0 {
				newPrice = p
			}
		}

		// Размер.
		newSize := ""
		if sizeIdx >= 0 && sizeIdx < len(record) {
			newSize = cleanCell(record[sizeIdx])
		}

		// Обновляем или создаём.
		found := false
		for i, p := range s.db.Plants {
			if strings.EqualFold(p.Name, name) {
				if newQty >= 0 {
					s.db.Plants[i].Qty = newQty
				}
				if newPrice > 0 {
					s.db.Plants[i].Price = newPrice
				}
				if newSize != "" {
					s.db.Plants[i].Size = newSize
				}
				found = true
				res.Imported++
				break
			}
		}
		if !found {
			qty := 0
			if newQty >= 0 {
				qty = newQty
			}
			price := 0.0
			if newPrice > 0 {
				price = newPrice
			}
			s.db.Plants = append(s.db.Plants, models.Plant{
				ID:       s.nextID(),
				Name:     name,
				Category: category,
				Size:     newSize,
				Price:    price,
				Qty:      qty,
			})
			res.Imported++
		}
	}

	res.Total = len(s.db.Plants)
	return res, s.saveLocked()
}

// parseNum — парсит число в русском формате ("25,50", "1 000").
func parseNum(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}

// cleanCell — убирает BOM и крайние пробелы.
func cleanCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")
	return s
}

// isHeaderOrTitle — true, если строка похожа на заголовок или название блока.
func isHeaderOrTitle(record []string) bool {
	if len(record) == 0 {
		return false
	}
	first := strings.ToLower(cleanCell(record[0]))
	if strings.Contains(first, "растени") || strings.Contains(first, "прайс") ||
		strings.Contains(first, "список") || first == "№" || first == "#" {
		return true
	}
	for _, cell := range record {
		if strings.Contains(strings.ToLower(cleanCell(cell)), "наименование") {
			return true
		}
	}
	return false
}

// CountPlants — сколько растений сейчас в каталоге (нужно после ImportCSV).
func (s *Store) CountPlants() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.db.Plants)
}
