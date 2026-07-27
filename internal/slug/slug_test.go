package slug

import (
	"bytes"
	"regexp"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "ACME Corp", "acme-corp"},
		{"umlaut ae", "Mäkelä", "maekelae"},
		{"umlaut oe", "Sjöberg Sörström", "sjoeberg-soerstroem"},
		{"umlaut ue", "Türkü Büyük", "tuerkue-bueyuek"},
		{"eszett", "Maßex", "massex"},
		{"mixed umlauts", "Häggkvist-Nyqvist", "haeggkvist-nyqvist"},
		{"symbols become dash", "ACME & Co. KG!!", "acme-co-kg"},
		{"collapses repeats", "a---b   c", "a-b-c"},
		{"trims edges", "--acme--", "acme"},
		{"digits kept", "Branch 42", "branch-42"},
		{"already lowercase dashed", "acme-corp", "acme-corp"},
		{"empty becomes board", "", "board"},
		{"only symbols becomes board", "!!!???", "board"},
		{"only umlauts", "ÄÖÜSS", "aeoeuess"},
		{"truncates to 32 runes", "abcdefghijklmnopqrstuvwxyz0123456789", "abcdefghijklmnopqrstuvwxyz012345"},
		{"truncation trims trailing dash", "abcdefghijklmnopqrstuvwxyzabcde-fghi", "abcdefghijklmnopqrstuvwxyzabcde"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.in)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerator_For_Deterministic(t *testing.T) {
	src := bytes.NewReader([]byte{0, 1, 2, 3, 4, 5})
	g := New(src)

	got, err := g.For("ACME Corp")
	if err != nil {
		t.Fatalf("For() error = %v", err)
	}

	want := "acme-corp-012345"
	if got != want {
		t.Errorf("For() = %q, want %q", got, want)
	}
}

func TestGenerator_For_ExhaustedEntropy(t *testing.T) {
	src := bytes.NewReader([]byte{0, 1})
	g := New(src)

	if _, err := g.For("ACME Corp"); err == nil {
		t.Fatal("For() expected error when entropy source runs out, got nil")
	}
}

var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func FuzzSlugify(f *testing.F) {
	seeds := []string{
		"ACME Corp", "Mäkelä Oy", "", "!!!", "---", "Ä Ö Ü ß",
		"already-a-slug", "123 456", "a b c d e f",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, in string) {
		got := Slugify(in)
		if got == "" {
			t.Fatalf("Slugify(%q) returned empty string", in)
		}
		if !slugPattern.MatchString(got) {
			t.Fatalf("Slugify(%q) = %q, does not match %s", in, got, slugPattern.String())
		}
	})
}
