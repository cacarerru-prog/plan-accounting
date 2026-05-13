// internal/storage/employees_test.go — тесты операций над сотрудниками.
package storage

import (
	"testing"

	"plants-app/internal/models"
)

func TestCreateEmployee_Basic(t *testing.T) {
	s := newTestStore(t)

	e := &models.Employee{Name: "Александр", Percent: 50}
	if err := s.CreateEmployee(e); err != nil {
		t.Fatalf("CreateEmployee: %v", err)
	}
	if e.ID == 0 {
		t.Error("ID должен быть проставлен после создания")
	}
}

func TestListEmployees(t *testing.T) {
	s := newTestStore(t)

	_ = s.CreateEmployee(&models.Employee{Name: "Елена", Percent: 25})
	_ = s.CreateEmployee(&models.Employee{Name: "Данила", Percent: 25})

	list := s.ListEmployees()
	if len(list) != 2 {
		t.Errorf("ожидаем 2 сотрудника, получили %d", len(list))
	}
	// Убеждаемся, что возвращается копия — мутация не влияет на store.
	list[0].Name = "mutated"
	if s.ListEmployees()[0].Name == "mutated" {
		t.Error("ListEmployees должен возвращать копию, а не ссылку")
	}
}

func TestUpdateEmployee(t *testing.T) {
	s := newTestStore(t)

	e := &models.Employee{Name: "Александр", Percent: 50}
	_ = s.CreateEmployee(e)

	updated := &models.Employee{Name: "Александр К.", Percent: 60}
	if err := s.UpdateEmployee(e.ID, updated); err != nil {
		t.Fatalf("UpdateEmployee: %v", err)
	}

	list := s.ListEmployees()
	if list[0].Name != "Александр К." || list[0].Percent != 60 {
		t.Errorf("после обновления: %+v", list[0])
	}
	// ID должен сохраниться.
	if list[0].ID != e.ID {
		t.Errorf("ID после обновления изменился: было %d, стало %d", e.ID, list[0].ID)
	}
}

func TestUpdateEmployee_NotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.UpdateEmployee(999, &models.Employee{Name: "X", Percent: 10})
	if err != ErrEmployeeNotFound {
		t.Errorf("ожидаем ErrEmployeeNotFound, получили %v", err)
	}
}

func TestDeleteEmployee(t *testing.T) {
	s := newTestStore(t)

	e := &models.Employee{Name: "Данила", Percent: 25}
	_ = s.CreateEmployee(e)

	if err := s.DeleteEmployee(e.ID); err != nil {
		t.Fatalf("DeleteEmployee: %v", err)
	}
	if len(s.ListEmployees()) != 0 {
		t.Error("после удаления список должен быть пуст")
	}
}

func TestDeleteEmployee_NotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteEmployee(999)
	if err != ErrEmployeeNotFound {
		t.Errorf("ожидаем ErrEmployeeNotFound, получили %v", err)
	}
}
