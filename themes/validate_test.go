package themes_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stiriacus/vitrine/themes"
)

// TestLocalThemes discovers theme directories next to the embedded "plain"
// theme and validates each one. It skips "plain" (already covered by
// TestBoardGolden and friends) and ignores dot-prefixed directories.
//
// The test is designed to pass in CI (where no local themes exist) and
// surface problems during local theme development before the author starts
// vitrine or pushes changes.
//
// Run just this test with:
//
//	go test ./themes/ -run TestLocalThemes -v
func TestLocalThemes(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read themes directory: %v", err)
	}

	found := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip the embedded theme (tested separately) and hidden directories.
		if name == "plain" || strings.HasPrefix(name, ".") {
			continue
		}

		found++
		themePath := filepath.Join(".", name)
		t.Run(name, func(t *testing.T) {
			if err := themes.ValidateDir(themePath); err != nil {
				t.Errorf("theme %q is invalid:\n  %v\n\nFix the errors above, then re-run this test.", name, err)
			}
		})
	}

	if found == 0 {
		t.Log("No local themes found — nothing to validate. (This is expected in CI.)")
	}
}
