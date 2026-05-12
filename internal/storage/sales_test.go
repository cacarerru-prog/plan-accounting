// internal/storage/sales_test.go — тесты продаж со списанием склада.
package storage

import (
	"errors"
	"testing"

	"plants-app/internal/models"
)

func TestCreateSale_DeductsStock(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10, Price: 50})

	sale := &models.Sale{PlantName: "Туя", Qty: 3, Price: 50, Channel: "Рынок"}
	if err := s.CreateSale(sale); err != nil {
		t.Fatalf("CreateSale: %v", err)
	}
	if sale.Total != 150 {
		t.Errorf("total: ожидаем 150, получили %v", sale.Total)
	}
	plants := s.ListPlants()
	if plants[0].Qty != 7 {
		t.Errorf("остаток на складе: ожидаем 7, получили %d", plants[0].Qty)
	}
}

func TestCreateSale_PlantNotInStock(t *testing.T) {
	s := newTestStore(t)

	err := s.CreateSale(&models.Sale{PlantName: "Нет такого", Qty: 1, Price: 100})
	if !errors.Is(err, ErrPlantNotInStock) {
		t.Errorf("ожидаем ErrPlantNotInStock, получили %v", err)
	}
}

func TestCreateSale_InsufficientQty(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 2, Price: 50})

	err := s.CreateSale(&models.Sale{PlantName: "Туя", Qty: 5, Price: 50})
	var insuf *ErrInsufficientQty
	if !errors.As(err, &insuf) || insuf.Available != 2 {
		t.Errorf("ожидаем ErrInsufficientQty{Available:2}, получили %v", err)
	}
}

func TestCreateSale_SkipStock(t *testing.T) {
	s := newTestStore(t)

	// Услуга (нет на складе), но SkipStock=true → должно пройти без ошибки.
	sale := &models.Sale{PlantName: "Услуга монтажа", Qty: 1, Price: 200, SkipStock: true}
	if err := s.CreateSale(sale); err != nil {
		t.Errorf("SkipStock-продажа должна была пройти: %v", err)
	}
	if sale.Total != 200 {
		t.Errorf("total: ожидаем 200, получили %v", sale.Total)
	}
}

// БАГ, который я фиксил: удаление SkipStock-продажи возвращало qty на склад.
func TestDeleteSale_SkipStockDoesNotRestoreQty(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 5, Price: 50})

	sale := &models.Sale{PlantName: "Туя", Qty: 999, Price: 200, SkipStock: true}
	if err := s.CreateSale(sale); err != nil {
		t.Fatalf("CreateSale: %v", err)
	}

	beforeQty := s.ListPlants()[0].Qty
	if err := s.DeleteSale(sale.ID); err != nil {
		t.Fatalf("DeleteSale: %v", err)
	}
	afterQty := s.ListPlants()[0].Qty

	if beforeQty != afterQty {
		t.Errorf("SkipStock-продажа НЕ должна была изменить склад при удалении: до=%d, после=%d",
			beforeQty, afterQty)
	}
}

// Обычная продажа при удалении должна вернуть qty на склад.
func TestDeleteSale_RestoresQty(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 10, Price: 50})

	sale := &models.Sale{PlantName: "Туя", Qty: 3, Price: 50}
	_ = s.CreateSale(sale)

	if s.ListPlants()[0].Qty != 7 {
		t.Fatalf("предусловие: ожидаем 7 шт. после продажи")
	}

	if err := s.DeleteSale(sale.ID); err != nil {
		t.Fatalf("DeleteSale: %v", err)
	}
	if s.ListPlants()[0].Qty != 10 {
		t.Errorf("после удаления qty не вернулся: получили %d", s.ListPlants()[0].Qty)
	}
}

func TestCreateSale_DefaultDate(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreatePlant(&models.Plant{Name: "Туя", Qty: 5})

	sale := &models.Sale{PlantName: "Туя", Qty: 1, Price: 100}
	_ = s.CreateSale(sale)

	if sale.Date == "" {
		t.Error("Date должен быть автозаполнен")
	}
}
