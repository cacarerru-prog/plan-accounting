// internal/storage/import_csv_test.go — тесты импорта прайс-листа из CSV.
package storage

import (
	"strings"
	"testing"

	"plants-app/internal/models"
)

func TestParseNum(t *testing.T) {
	// Русский формат: запятая, неразрывный пробел, обычный пробел.
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"25,50", 25.5, true},
		{"1 000", 1000, true},
		{"1 000", 1000, true}, // неразрывный пробел
		{"  42  ", 42, true},
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, err := parseNum(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("parseNum(%q): want %v, got %v (err=%v)", c.in, c.want, got, err)
		}
		if !c.ok && err == nil {
			t.Errorf("parseNum(%q): ожидаем ошибку, получили %v", c.in, got)
		}
	}
}

func TestCleanCell(t *testing.T) {
	// BOM в начале убирается, пробелы по краям тоже.
	if got := cleanCell("\xef\xbb\xbfТуя"); got != "Туя" {
		t.Errorf("cleanCell BOM: ожидаем 'Туя', получили %q", got)
	}
	if got := cleanCell("  Туя  "); got != "Туя" {
		t.Errorf("cleanCell spaces: ожидаем 'Туя', получили %q", got)
	}
	if got := cleanCell(""); got != "" {
		t.Errorf("cleanCell empty: ожидаем '', получили %q", got)
	}
}

func TestIsHeaderOrTitle(t *testing.T) {
	cases := []struct {
		name   string
		record []string
		want   bool
	}{
		{"пусто", []string{}, false},
		{"растения", []string{"Растения", "", ""}, true},
		{"прайс", []string{"Прайс-лист 2026"}, true},
		{"наименование во второй", []string{"№", "Наименование", "Цена"}, true},
		{"обычная строка", []string{"Туя", "10", "50"}, false},
	}
	for _, c := range cases {
		if got := isHeaderOrTitle(c.record); got != c.want {
			t.Errorf("%s: want %v, got %v", c.name, c.want, got)
		}
	}
}

func TestImportCSV_NewPlants(t *testing.T) {
	s := newTestStore(t)

	csvData := "Наименование,Количество,Цена\n" +
		"Туя,10,50\n" +
		"Самшит,5,80\n"

	res, err := s.ImportCSV(strings.NewReader(csvData), "Хвойные")
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if res.Imported != 2 {
		t.Errorf("Imported: ожидаем 2, получили %d", res.Imported)
	}
	if s.CountPlants() != 2 {
		t.Errorf("CountPlants: ожидаем 2, получили %d", s.CountPlants())
	}
}

func TestImportCSV_UpdatesExisting(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 5, Price: 30})

	// Тот же набор колонок, но обновляем количество и цену.
	csvData := "Наименование,Количество,Цена\nТуя,20,55\n"

	res, err := s.ImportCSV(strings.NewReader(csvData), "Хвойные")
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if res.Imported != 1 {
		t.Errorf("Imported: ожидаем 1, получили %d", res.Imported)
	}
	p := s.ListPlants()[0]
	if p.Qty != 20 || p.Price != 55 {
		t.Errorf("обновление не применилось: %+v", p)
	}
}

func TestImportCSV_DecimalCommaAndSpaces(t *testing.T) {
	s := newTestStore(t)

	// Цена "1 234,50" — типичный формат Google Sheets.
	csvData := "Наименование,Количество,Цена\n" +
		"Туя западная,15,\"1 234,50\"\n"

	if _, err := s.ImportCSV(strings.NewReader(csvData), ""); err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	p := s.ListPlants()[0]
	if p.Price != 1234.5 {
		t.Errorf("ожидаем цену 1234.5, получили %v", p.Price)
	}
}

func TestImportCSV_SkipsTotals(t *testing.T) {
	s := newTestStore(t)

	csvData := "Наименование,Количество,Цена\n" +
		"Туя,10,50\n" +
		"Итого,10,50\n" +
		"Всего,10,50\n"

	res, err := s.ImportCSV(strings.NewReader(csvData), "")
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if res.Imported != 1 {
		t.Errorf("должна была импортироваться только Туя, получили %d", res.Imported)
	}
}

func TestImportCSV_DefaultCategory(t *testing.T) {
	s := newTestStore(t)

	csvData := "Наименование,Количество,Цена\nРоза,7,150\n"
	if _, err := s.ImportCSV(strings.NewReader(csvData), ""); err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	p := s.ListPlants()[0]
	if p.Category != "Лиственные" {
		t.Errorf("ожидаем дефолтную категорию 'Лиственные', получили %q", p.Category)
	}
}

func TestImportCSV_WithSizeColumn(t *testing.T) {
	s := newTestStore(t)

	csvData := "Наименование,Размер,Количество,Цена\n" +
		"Туя,C3,10,50\n"

	if _, err := s.ImportCSV(strings.NewReader(csvData), "Хвойные"); err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	p := s.ListPlants()[0]
	if p.Size != "C3" {
		t.Errorf("ожидаем размер 'C3', получили %q", p.Size)
	}
}

func TestImportCSV_SkipsHeaderRow(t *testing.T) {
	s := newTestStore(t)

	csvData := "Прайс-лист питомника\n" +
		"Наименование,Количество,Цена\n" +
		"Туя,10,50\n"

	res, err := s.ImportCSV(strings.NewReader(csvData), "")
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if res.Imported != 1 {
		t.Errorf("ожидаем 1 импортированную, получили %d", res.Imported)
	}
}
