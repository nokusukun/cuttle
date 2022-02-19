package locust

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

type Sample struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Name    string    `json:"name"`
	Ppu     float64   `json:"ppu"`
	Batters Batters   `json:"batters"`
	Topping []Topping `json:"topping"`
}

type Batters struct {
	Batter []Topping `json:"batter"`
}

type Topping struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}


func TestGenerateTypeMap(t *testing.T) {
	s := Sample{}
	result := GenerateTypeMap(reflect.TypeOf(s))
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}