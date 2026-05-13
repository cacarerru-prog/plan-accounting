// internal/models/models_test.go — тесты дефолтных значений доменных моделей.
package models

import "testing"

func TestDefaultEmployees(t *testing.T) {
	got := DefaultEmployees()
	if len(got) != 3 {
		t.Fatalf("ожидаем 3 дефолтных сотрудника, получили %d", len(got))
	}

	// Сумма процентов должна быть ровно 100 — иначе расчёт зарплат сломается.
	var sum float64
	for _, e := range got {
		sum += e.Percent
	}
	if sum != 100 {
		t.Errorf("суммарный процент должен быть 100, получили %v", sum)
	}

	// ID-шники у дефолтных сотрудников должны быть 1, 2, 3.
	for i, e := range got {
		if e.ID != i+1 {
			t.Errorf("сотрудник[%d]: ожидаем ID=%d, получили %d", i, i+1, e.ID)
		}
		if e.Name == "" {
			t.Errorf("сотрудник[%d] без имени: %+v", i, e)
		}
	}
}
