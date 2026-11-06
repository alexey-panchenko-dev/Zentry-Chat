package httpapi

import "testing"

func TestNormalizeContactPhone(t *testing.T) {
	for _, v := range []struct {
		in, out string
		ok      bool
	}{{"", "", true}, {"   ", "", true}, {"+7 (777) 123-45-67", "+77771234567", true}, {"12345678", "+12345678", true}, {"+01234567", "", false}, {"00000000", "", false}, {"+123456789012345", "+123456789012345", true}, {"+1234567", "", false}, {"+1234567890123456", "", false}, {"123+45678", "", false}, {"++12345678", "", false}, {"+7 call me 7771234567", "", false}, {"１２３４５６７８", "", false}} {
		got, e := normalizeContactPhone(v.in)
		if (e == nil) != v.ok || got != v.out {
			t.Errorf("%q => %q, %v", v.in, got, e)
		}
	}
}
