package game

import (
	"strings"
	"testing"
)

func TestSanitizePlayerName(t *testing.T) {
	const fallback = "Player 7"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: fallback},
		{name: "whitespace only", in: "   \t ", want: fallback},
		{name: "below minimum", in: "Al", want: fallback},
		{name: "at minimum", in: "Bob", want: "Bob"},
		{name: "surrounding whitespace trimmed", in: "  Alice  ", want: "Alice"},
		{name: "inner space kept", in: "Jo Jo", want: "Jo Jo"},
		{name: "at maximum", in: strings.Repeat("a", maxNameRunes), want: strings.Repeat("a", maxNameRunes)},
		{name: "over maximum truncated", in: strings.Repeat("a", maxNameRunes+4), want: strings.Repeat("a", maxNameRunes)},
		{name: "multibyte counted as runes", in: strings.Repeat("ż", maxNameRunes+4), want: strings.Repeat("ż", maxNameRunes)},
		{name: "control characters stripped", in: "Al\x00i\nce", want: "Alice"},
		{name: "minimum checked after stripping", in: "A\n\n\n\nB", want: fallback},
		{name: "zero-width only", in: "​​​", want: fallback},
		{name: "bidi override stripped", in: "Bob‮evil", want: "Bobevil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizePlayerName(tt.in, 7); got != tt.want {
				t.Fatalf("sanitizePlayerName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
