package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	normStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).MarginBottom(1)
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
	s := titleStyle.Render(p.title) + "\n"
	for i, c := range p.choices {
		if i == p.cursor {
			s += selStyle.Render("▶ "+c) + "\n"
		} else {
			s += normStyle.Render("  "+c) + "\n"
		}
	}
	return s + normStyle.Render("\n↑/↓  enter select")
}

func pick(title string, choices []string) (string, error) {
	result, err := tea.NewProgram(picker{title: title, choices: choices}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	if chosen := result.(picker).chosen; chosen != "" {
		return chosen, nil
	}
	return "", fmt.Errorf("cancelled")
}
