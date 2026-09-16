package magetools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseToolInfo(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    toolInfoResult
		wantErr bool
	}{
		{
			name:    "tool with its providing module",
			fixture: "golangci-lint.json",
			want: toolInfoResult{
				Package: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
				Module:  "github.com/golangci/golangci-lint/v2",
				Version: "v2.12.2",
			},
		},
		{
			name:    "longest matching require wins",
			fixture: "nested-module.json",
			want: toolInfoResult{
				Package: "golang.org/x/vuln/cmd/govulncheck",
				Module:  "golang.org/x/vuln",
				Version: "v1.1.4",
			},
		},
		{
			name:    "no tool directive is an error",
			fixture: "no-tool.json",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", "modfiles", tt.fixture))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			got, err := parseToolInfo(content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseToolInfo(%s) = %+v, want error", tt.fixture, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToolInfo(%s): %v", tt.fixture, err)
			}
			if got != tt.want {
				t.Errorf("parseToolInfo(%s) = %+v, want %+v", tt.fixture, got, tt.want)
			}
		})
	}
}

func TestParseToolInfoInvalidJSON(t *testing.T) {
	if _, err := parseToolInfo([]byte("not json")); err == nil {
		t.Fatal("parseToolInfo(invalid) = nil error, want error")
	}
}
