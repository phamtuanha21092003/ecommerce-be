package base

import (
	"strings"
	"testing"

	jsonv2 "encoding/json/v2"

	"github.com/shopspring/decimal"
)

func TestOptionalJSON(t *testing.T) {
	var body struct {
		Name        Optional[string]          `json:"name"`
		Description Optional[string]          `json:"description"`
		Price       Optional[decimal.Decimal] `json:"price"`
	}

	err := jsonv2.UnmarshalRead(strings.NewReader(`{"description":null,"price":"19.99"}`), &body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body.Name.Set || body.Name.Null {
		t.Fatalf("omitted name was marked present: %+v", body.Name)
	}
	if !body.Description.Set || !body.Description.Null {
		t.Fatalf("null description = %+v", body.Description)
	}
	if !body.Price.Set || body.Price.Null {
		t.Fatalf("price flags = %+v", body.Price)
	}
	if !body.Price.Value.Equal(decimal.RequireFromString("19.99")) {
		t.Fatalf("price = %s", body.Price.Value)
	}
}

func TestOptionalRejectsWrongType(t *testing.T) {
	var name Optional[string]
	if err := name.UnmarshalJSON([]byte(`1`)); err == nil {
		t.Fatal("expected type error")
	}
}
