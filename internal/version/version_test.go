package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.2.0", 0},
		{"v1.2.0", "1.2.0", 0},
		{"1.10.0", "1.9.3", 1},
		{"1.2", "1.2.1", -1},
		{"2.0.0", "10.0.0", -1},
		{"1.2.3-beta.1", "1.2.3", 0},
		{"1.2.3+build", "1.2.3", 0},
		{"dev", "1.0.0", -1},
		{"", "0.0.1", -1},
	}
	for _, tc := range cases {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, quiero %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestStringTrimsPrefix(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v1.3.0"
	if got := String(); got != "1.3.0" {
		t.Fatalf("String() = %q, quiero 1.3.0", got)
	}

	Version = "  "
	if got := String(); got != "dev" {
		t.Fatalf("String() = %q, quiero dev", got)
	}
}
