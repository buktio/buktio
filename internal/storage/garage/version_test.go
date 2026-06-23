package garage

import (
	"errors"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := map[string]Version{
		"v2.3.0":     {2, 3, 0},
		"2.3.0":      {2, 3, 0},
		"2.3.0-rc1":  {2, 3, 0},
		"2.3.0+meta": {2, 3, 0},
		"2.3":        {2, 3, 0},
		"v1.0.4":     {1, 0, 4},
	}
	for in, want := range cases {
		got, err := ParseVersion(in)
		if err != nil {
			t.Errorf("ParseVersion(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseVersion(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCheckSupported(t *testing.T) {
	if err := CheckSupported(Version{2, 0, 0}); err != nil {
		t.Errorf("2.0.0 should be supported: %v", err)
	}
	if err := CheckSupported(Version{2, 3, 0}); err != nil {
		t.Errorf("2.3.0 should be supported: %v", err)
	}
	err := CheckSupported(Version{1, 0, 4})
	if err == nil || !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("1.0.4 should be unsupported, got %v", err)
	}
}

func TestSupportsSingleNode(t *testing.T) {
	if !SupportsSingleNode(Version{2, 3, 0}) {
		t.Error("2.3.0 should support --single-node")
	}
	if SupportsSingleNode(Version{2, 2, 9}) {
		t.Error("2.2.9 should NOT support --single-node")
	}
	if !SupportsSingleNode(Version{3, 0, 0}) {
		t.Error("3.0.0 should support --single-node")
	}
}
