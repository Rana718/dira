package helper

import "testing"

func TestPadUsesTerminalDisplayWidth(t *testing.T) {
	tests := []struct {
		name, input, want string
		width             int
	}{
		{name: "ascii", input: "abc", width: 5, want: "abc  "},
		{name: "wide unicode", input: "界", width: 4, want: "界  "},
		{name: "already wide", input: "abcdef", width: 3, want: "abcdef"},
		{name: "zero width", input: "abc", width: 0, want: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Pad(tt.input, tt.width); got != tt.want {
				t.Fatalf("Pad(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestWidthUsesRenderedCellCount(t *testing.T) {
	if got := Width("A界B"); got != 4 {
		t.Fatalf("Width() = %d, want 4", got)
	}
}
