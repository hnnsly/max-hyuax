package geo_test

import (
	"testing"

	"dommax/internal/storage/geo"
)

func TestCleanDistrictName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"район Зябликово", "Зябликово"},
		{"Тверской район", "Тверской"},
		{"муниципальный округ Хамовники", "Хамовники"},
		{"поселение Сосенское", "Сосенское"},
		{"Басманный", "Басманный"},
	}
	for _, tc := range tests {
		got := geo.CleanDistrictName(tc.in)
		if got != tc.want {
			t.Errorf("CleanDistrictName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
