package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pickerSel   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	pickerNorm  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	pickerTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).MarginBottom(1)
)

type picker struct {
	title   string
	choices []string
	cursor  int
	chosen  string
}

func (p picker) Init() tea.Cmd { return nil }

func (p picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.choices)-1 {
				p.cursor++
			}
		case "enter", " ":
			p.chosen = p.choices[p.cursor]
			return p, tea.Quit
		case "ctrl+c", "q":
			return p, tea.Quit
		}
	}
	return p, nil
}

func (p picker) View() string {
	s := pickerTitle.Render(p.title) + "\n"
	for i, c := range p.choices {
		if i == p.cursor {
			s += pickerSel.Render("▶ "+c) + "\n"
		} else {
			s += pickerNorm.Render("  "+c) + "\n"
		}
	}
	return s + pickerNorm.Render("\n↑/↓  enter select")
}

// Pick runs a TUI picker and returns the chosen item or error if cancelled.
func Pick(title string, choices []string) (string, error) {
	result, err := tea.NewProgram(picker{title: title, choices: choices}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	if chosen := result.(picker).chosen; chosen != "" {
		return chosen, nil
	}
	return "", fmt.Errorf("cancelled")
}

// ColorizeLogs adds color to log lines based on level keywords.
func ColorizeLogs(raw string) string {
	var out string
	for _, line := range splitLines(raw) {
		lower := toLower(line)
		switch {
		case contains(lower, "error", "fatal", "panic"):
			out += Red.Render(line)
		case contains(lower, "warn"):
			out += Yellow.Render(line)
		case contains(lower, "info"):
			out += Green.Render(line)
		default:
			out += Value.Render(line)
		}
		out += "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
