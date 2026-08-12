package tui

import "github.com/charmbracelet/lipgloss"

var (
	Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	Border = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))

	Green  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	Red    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	Yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	Blue   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	Value = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	Dim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Label = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	Help  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)
