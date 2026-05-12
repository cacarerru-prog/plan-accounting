// Package models — доменные структуры приложения.
//
// Эти типы используются и в хранилище (storage), и в HTTP-обработчиках (api).
// Здесь нет никакой логики работы с файлом или HTTP — только данные.
package models

// Plant — растение в каталоге склада.
type Plant struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Size     string  `json:"size"`
	Price    float64 `json:"price"`
	Qty      int     `json:"qty"`
}

// Sale — розничная продажа.
// SkipStock = true означает «прочее» (услуга или товар не из каталога) —
// в этом случае склад не проверяется при создании и не возвращается при удалении.
type Sale struct {
	ID        int     `json:"id"`
	PlantName string  `json:"plant_name"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	Total     float64 `json:"total"`
	Channel   string  `json:"channel"`
	Date      string  `json:"date"`
	SkipStock bool    `json:"skip_stock"`
}

// Expense — расход (закупка, аренда, транспорт и т.п.).
type Expense struct {
	ID          int     `json:"id"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
}

// Employee — сотрудник, получающий процент от прибыли.
type Employee struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

// ProjectPlant — одно растение в составе проекта озеленения.
type ProjectPlant struct {
	PlantName string  `json:"plant_name"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	Total     float64 `json:"total"`
}

// Project — заказ на озеленение заведения.
// Отличается от обычной продажи тем, что содержит:
//   - привязку к клиенту (Client)
//   - список использованных растений
//   - стоимость работы (LaborCost)
// При создании растения автоматически списываются со склада.
type Project struct {
	ID        int            `json:"id"`
	Client    string         `json:"client"`
	Date      string         `json:"date"`
	Channel   string         `json:"channel"`
	Plants    []ProjectPlant `json:"plants"`
	LaborCost float64        `json:"labor_cost"`
	Total     float64        `json:"total"`
	Notes     string         `json:"notes"`
}

// DB — снимок всех данных приложения, который сериализуется в data.json.
type DB struct {
	Plants    []Plant    `json:"plants"`
	Sales     []Sale     `json:"sales"`
	Expenses  []Expense  `json:"expenses"`
	Employees []Employee `json:"employees"`
	Projects  []Project  `json:"projects"`
	NextID    int        `json:"next_id"`
}

// DefaultEmployees — стандартный состав, если в data.json никого нет.
func DefaultEmployees() []Employee {
	return []Employee{
		{ID: 1, Name: "Елена", Percent: 50},
		{ID: 2, Name: "Александр", Percent: 25},
		{ID: 3, Name: "Данила", Percent: 25},
	}
}
