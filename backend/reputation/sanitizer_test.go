package reputation

import (
	"testing"
)

func TestSanitizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Valid email", "user@gmail.com", "user@gmail.com"},
		{"Whitespace and uppercase", "  USER@gMAil.Com  ", "user@gmail.com"},
		{"Common typo gamil", "user@gamil.com", "user@gmail.com"},
		{"Common typo hotmial", "test@hotmial.com", "test@hotmail.com"},
		{"Common typo outlok", "info@outlok.com", "info@outlook.com"},
		{"Common typo yaho", "contact@yaho.com", "contact@yahoo.com"},
		{"Invalid format no domain", "user@", "user@"},
		{"Invalid format no at", "usergmail.com", "usergmail.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sanitize(tt.input)
			if result.Fixed != tt.expected {
				t.Errorf("Sanitize(%q).Fixed = %q; want %q", tt.input, result.Fixed, tt.expected)
			}
		})
	}
}
