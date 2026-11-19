package web

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestColorContractParityAndInvariantArtwork(t *testing.T) {
	data, err := os.ReadFile("colors.json")
	if err != nil {
		t.Fatal(err)
	}
	var registry struct {
		Colors map[ColorKey]struct{ Light, Dark, Category string }
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	if len(registry.Colors) != len(colorTokens) {
		t.Fatal("Go color output is stale; regenerate from colors.json")
	}
	for key, entry := range registry.Colors {
		if Color(key, Light) != entry.Light || Color(key, Dark) != entry.Dark {
			t.Fatalf("color %s differs from canonical registry", key)
		}
		if (entry.Category == "invariant" || entry.Category == "artwork") && entry.Light != entry.Dark {
			t.Fatalf("%s changes artwork or QR/media identity with the theme", key)
		}
	}
	for _, required := range []string{":root:not([data-theme])", "@media print", ":root:root", "prefers-contrast:more"} {
		if !strings.Contains(ColorStyles(), required) {
			t.Fatalf("isolated consent/share documents lose theme fallback: %s", required)
		}
	}
}
