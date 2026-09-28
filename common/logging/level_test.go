package logging

import (
	"log/slog"
	"testing"
)

func TestResolveLevel(t *testing.T) {
	tests := []struct {
		configured string
		env        string
		want       slog.Level
		wantOk     bool
	}{
		{"", "", slog.LevelInfo, true},
		{"debug", "", slog.LevelDebug, true},
		{"WARNING", "", slog.LevelWarn, true},
		{"error", "", slog.LevelError, true},
		{"bogus", "", slog.LevelInfo, false},
		{"error", "debug", slog.LevelDebug, true},
		{"debug", "bogus", slog.LevelInfo, false},
	}
	for _, tt := range tests {
		t.Setenv("LOG_LEVEL", tt.env)
		got, _, ok := ResolveLevel(tt.configured)
		if got != tt.want || ok != tt.wantOk {
			t.Errorf("ResolveLevel(%q) with LOG_LEVEL=%q = %v, %v; want %v, %v",
				tt.configured, tt.env, got, ok, tt.want, tt.wantOk)
		}
	}
}
