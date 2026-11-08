package app

import "testing"

func TestIsRightHalf(t *testing.T) {
	cases := []struct {
		half  int
		manga bool
		want  bool
	}{
		{0, false, false}, // Occidental, primera mitad: izquierda
		{1, false, true},  // Occidental, segunda mitad: derecha
		{0, true, true},   // Manga, primera mitad: derecha
		{1, true, false},  // Manga, segunda mitad: izquierda
	}

	for _, tc := range cases {
		if got := isRightHalf(tc.half, tc.manga); got != tc.want {
			t.Errorf("isRightHalf(%d, manga=%v) = %v, quiero %v", tc.half, tc.manga, got, tc.want)
		}
	}
}
