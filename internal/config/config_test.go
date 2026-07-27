package config

import (
	"strings"
	"testing"
)

const validSecret = "01234567890123456789012345678901" // 33 chars

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("VITRINE_WEBHOOK_SECRET", validSecret)
	t.Setenv("VITRINE_BASE_URL", "https://boards.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := Config{
		Addr:          defaultAddr,
		DBPath:        defaultDBPath,
		WebhookSecret: validSecret,
		BaseURL:       "https://boards.example.com",
		Theme:         defaultTheme,
		LogLevel:      defaultLogLevel,
		Dev:           false,
		DemoPayload:   "",
		PoweredBy:     "MyCompany",
	}
	if cfg != want {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("VITRINE_WEBHOOK_SECRET", validSecret)
	t.Setenv("VITRINE_BASE_URL", "https://boards.example.com")
	t.Setenv("VITRINE_ADDR", ":9090")
	t.Setenv("VITRINE_DB_PATH", "/data/vitrine.db")
	t.Setenv("VITRINE_THEME", "custom")
	t.Setenv("VITRINE_LOG_LEVEL", "debug")
	t.Setenv("VITRINE_DEV", "true")
	t.Setenv("VITRINE_DEMO_PAYLOAD", "testdata/example_payload.json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := Config{
		Addr:          ":9090",
		DBPath:        "/data/vitrine.db",
		WebhookSecret: validSecret,
		BaseURL:       "https://boards.example.com",
		Theme:         "custom",
		LogLevel:      "debug",
		Dev:           true,
		DemoPayload:   "testdata/example_payload.json",
		PoweredBy:     "MyCompany",
	}
	if cfg != want {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_RequiredFieldErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing secret",
			env:     map[string]string{"VITRINE_BASE_URL": "https://boards.example.com"},
			wantErr: "VITRINE_WEBHOOK_SECRET is required",
		},
		{
			name: "secret too short",
			env: map[string]string{
				"VITRINE_WEBHOOK_SECRET": "too-short",
				"VITRINE_BASE_URL":       "https://boards.example.com",
			},
			wantErr: "VITRINE_WEBHOOK_SECRET must be at least 32 characters",
		},
		{
			name:    "missing base url",
			env:     map[string]string{"VITRINE_WEBHOOK_SECRET": validSecret},
			wantErr: "VITRINE_BASE_URL is required",
		},
		{
			name: "invalid dev flag",
			env: map[string]string{
				"VITRINE_WEBHOOK_SECRET": validSecret,
				"VITRINE_BASE_URL":       "https://boards.example.com",
				"VITRINE_DEV":            "not-a-bool",
			},
			wantErr: "VITRINE_DEV must be a valid boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() returned no error, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
