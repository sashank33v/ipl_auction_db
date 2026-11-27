package auction

import "testing"

func TestValidateRoomRules(t *testing.T) {
	cases := []struct {
		name, purse, inc   string
		min, max, overseas int
		valid              bool
	}{
		{"defaults", "", "", 0, 0, 0, true},
		{"approved", "120.00", "0.10", 18, 25, 8, true},
		{"equivalent decimals", "120", ".1", 18, 25, 8, true},
		{"wrong purse", "121", "0.10", 18, 25, 8, false},
		{"wrong increment", "120", "0.01", 18, 25, 8, false},
		{"wrong min", "120", "0.10", 17, 25, 8, false},
		{"wrong max", "120", "0.10", 18, 24, 8, false},
		{"wrong overseas", "120", "0.10", 18, 25, 9, false},
		{"negative", "-120", "0.10", 18, 25, 8, false},
		{"NaN", "NaN", "0.10", 18, 25, 8, false},
		{"fraction", "120/1", "0.10", 18, 25, 8, false},
		{"exponent", "12e1", "0.10", 18, 25, 8, false},
		{"rounding prohibited", "120.001", "0.10", 18, 25, 8, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidateRoomRules(c.purse, c.inc, c.min, c.max, c.overseas) == nil; got != c.valid {
				t.Fatalf("valid=%v; want %v", got, c.valid)
			}
		})
	}
}
