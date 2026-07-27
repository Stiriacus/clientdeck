package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Stiriacus/vitrine/internal/board"
)

func TestService_RenderBoard_BuildsViewModel(t *testing.T) {
	theme, bundle, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	svc := NewService(theme, bundle, "TestCompany")

	v := board.CustomerView{
		ClientName: "ACME Corp",
		Products: []board.Product{
			{Category: "Printers", Title: "Brother HL-L2375DW"},
		},
	}

	var buf bytes.Buffer
	if err := svc.RenderBoard(&buf, v); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	if !strings.Contains(buf.String(), "ACME Corp") {
		t.Fatalf("output missing client name: %s", buf.String())
	}
}

func TestService_RenderNotFound(t *testing.T) {
	theme, bundle, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	svc := NewService(theme, bundle, "TestCompany")

	var buf bytes.Buffer
	if err := svc.RenderNotFound(&buf, "en"); err != nil {
		t.Fatalf("RenderNotFound: %v", err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Fatalf("output = %s, want not-found content", buf.String())
	}
}
