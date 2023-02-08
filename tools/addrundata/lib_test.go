package main

import "testing"

func TestLessVersion(t *testing.T) {
	cases := []struct {
		x, y string
		want bool
	}{
		// Same as x < y using golang builtin string comparison.
		{"apple.1", "banana.1", true},
		{"banana.1", "apple.1", false},
		{"apple.1", "apple.1", false},
		{"1.apple", "1.banana", true},
		{"1.banana", "1.apple", false},
		{"1.apple", "1.apple", false},

		{"foo-1.1", "foo-1.2", true},
		{"foo-1.2", "foo-1.1", false},

		// These results differ from string comparison.
		{"foo-1a", "foo-10", true},
		{"foo-2.1", "foo-10.1", true},

		// Same as builtin string comparison but tricky for version comparison.
		{"foo-01.1", "foo-1.1", true},
	}

	for _, c := range cases {
		got := lessVersion(c.x, c.y)
		if got != c.want {
			t.Errorf("lessVersion(%q, %q) got %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
