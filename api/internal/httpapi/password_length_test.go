package httpapi

import (
	"strings"
	"testing"
)

func TestPasswordLength32(t *testing.T) {
	for _, n := range []int{6, 16, 24, 32} {
		if !validPassword("Aa1&" + strings.Repeat("x", n-4)) {
			t.Fatalf("valid %d character password rejected", n)
		}
	}
	for _, value := range []string{"Aa1&x", "Aa1&" + strings.Repeat("x", 29), "lowercase1&", "UPPERCASE1&", "NoDigits&", "NoSpecial1"} {
		if validPassword(value) {
			t.Fatal("invalid password accepted")
		}
	}
}
