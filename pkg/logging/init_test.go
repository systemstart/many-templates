package logging

import "testing"

func TestInitialize(t *testing.T) {
	tests := []struct {
		name      string
		logType   string
		level     string
		wantError bool
	}{
		{"json/info", JSON, "info", false},
		{"text/debug", Text, "debug", false},
		{"tint/warn", Tint, "warn", false},
		{"json/error", JSON, "error", false},
		{"invalid level", JSON, "bogus", true},
		{"unknown type", "unknown", "info", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Initialize(tt.logType, tt.level)
			if (err != nil) != tt.wantError {
				t.Errorf("Initialize(%q, %q) error = %v, wantError = %v", tt.logType, tt.level, err, tt.wantError)
			}
		})
	}
}
