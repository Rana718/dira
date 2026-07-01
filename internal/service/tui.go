package service

import (
	"fmt"
	"strings"

	"github.com/Rana718/dira/internal/helper"
	"github.com/Rana718/dira/internal/tui"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	svcHdr    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	svcSel    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	svcDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	svcGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	svcRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	svcYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	svcBlue   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	svcHelp   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	svcBorder = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	svcVal    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

type Mode int

const (
	ModeList Mode = iota
	ModeLogs
	ModeInfo
)

type ContentMsg struct{ Text, Err string }
type DoneMsg struct{ Err error }

type Model struct {
	All, Filtered []Entry
	Cursor        int
	Screen        Mode
	VP, ListVP    viewport.Model
	WinW, WinH    int
	Filter        string
	Search        bool
	Loading       bool
	Err           string
}

func NewModel() Model {
	svcs := List()
	m := Model{
		All:      svcs,
		Filtered: svcs,
		VP:       viewport.New(80, 20),
		ListVP:   viewport.New(80, 20),
	}
	m.rebuildList()
	return m
}

func (m *Model) applyFilter() {
	if m.Filter == "" {
		m.Filtered = m.All
	} else {
		f := strings.ToLower(m.Filter)
		var out []Entry
		for _, s := range m.All {
			if strings.Contains(strings.ToLower(s.Name), f) ||
				strings.Contains(strings.ToLower(s.Description), f) {
				out = append(out, s)
			}
		}
		m.Filtered = out
	}
	if m.Cursor >= len(m.Filtered) {
		m.Cursor = 0
	}
	m.rebuildList()
}

func (m *Model) selected() *Entry {
	if len(m.Filtered) == 0 || m.Cursor >= len(m.Filtered) {
		return nil
	}
	s := m.Filtered[m.Cursor]
	return &s
}

func (m *Model) rebuildList() {
	wName, wActive, wSub, wEnabled := 36, 10, 10, 10
	var sb strings.Builder
	for i, svc := range m.Filtered {
		cursor := "  "
		nameStyle := svcDim
		if i == m.Cursor {
			cursor = "▶ "
			nameStyle = svcSel
		}
		activeStyle := svcDim
		switch svc.Active {
		case "active":
			activeStyle = svcGreen
		case "failed":
			activeStyle = svcRed
		case "activating":
			activeStyle = svcYellow
		}
		subStyle := svcDim
		if svc.Sub == "running" {
			subStyle = svcGreen
		}
		enabledStyle := svcDim
		if svc.Enabled == "enabled" {
			enabledStyle = svcBlue
		}
		name := svc.Name
		if len([]rune(name)) > wName {
			name = string([]rune(name)[:wName-1]) + "…"
		}
		sb.WriteString(fmt.Sprintf("%s%s  %s  %s  %s\n",
			cursor,
			nameStyle.Render(helper.PadR(name, wName)),
			activeStyle.Render(helper.PadR(svc.Active, wActive)),
			subStyle.Render(helper.PadR(svc.Sub, wSub)),
			enabledStyle.Render(helper.PadR(svc.Enabled, wEnabled)),
		))
	}
	if len(m.Filtered) == 0 {
		sb.WriteString(svcDim.Render("  No services found") + "\n")
	}
	m.ListVP.SetContent(sb.String())
}

func (m *Model) ensureCursorVisible() {
	if m.ListVP.Height == 0 {
		return
	}
	if m.Cursor < m.ListVP.YOffset {
		m.ListVP.SetYOffset(m.Cursor)
	} else if m.Cursor >= m.ListVP.YOffset+m.ListVP.Height {
		m.ListVP.SetYOffset(m.Cursor - m.ListVP.Height + 1)
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.WinW, m.WinH = msg.Width, msg.Height
		m.VP.Width = msg.Width - 2
		m.VP.Height = msg.Height - 5
		m.ListVP.Width = msg.Width - 2
		m.ListVP.Height = msg.Height - 8
		m.rebuildList()
		return m, nil

	case ContentMsg:
		m.Loading = false
		m.Err = msg.Err
		m.VP.SetContent(msg.Text)
		m.VP.GotoTop()
		return m, nil

	case DoneMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err.Error()
		} else {
			m.Err = ""
			m.All = List()
			m.applyFilter()
		}
		m.rebuildList()
		return m, nil

	case tea.KeyMsg:
		if m.Screen != ModeList {
			return m.updateSubView(msg)
		}
		if m.Search {
			return m.updateSearch(msg)
		}
		switch msg.String() {
		case "pgup", "ctrl+u", "pgdn", "ctrl+d":
			var cmd tea.Cmd
			m.ListVP, cmd = m.ListVP.Update(msg)
			return m, cmd
		}
		return m.updateList(msg)

	case tea.MouseMsg:
		if m.Screen == ModeList {
			var cmd tea.Cmd
			m.ListVP, cmd = m.ListVP.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) updateSubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "q" || msg.String() == "esc" {
		m.Screen = ModeList
		m.Err = ""
		m.Loading = false
		m.VP.SetContent("")
		return m, nil
	}
	var cmd tea.Cmd
	m.VP, cmd = m.VP.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.Search = false
	case "backspace", "ctrl+h":
		if len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
			m.applyFilter()
		}
	default:
		if len(msg.String()) == 1 {
			m.Filter += msg.String()
			m.applyFilter()
		}
	}
	m.rebuildList()
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.selected()
	m.Err = ""
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			m.ensureCursorVisible()
		}
	case "down", "j":
		if m.Cursor < len(m.Filtered)-1 {
			m.Cursor++
			m.ensureCursorVisible()
		}
	case "/":
		m.Search = true
	case "esc":
		m.Filter = ""
		m.applyFilter()
	case "q", "ctrl+c":
		return m, tea.Quit
	case "l":
		if s == nil {
			break
		}
		m.Screen, m.Loading = ModeLogs, true
		name := s.Name
		return m, func() tea.Msg {
			raw, err := Logs(name, 200)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			return ContentMsg{Text: tui.ColorizeLogs(raw), Err: errStr}
		}
	case "i":
		if s == nil {
			break
		}
		m.Screen, m.Loading = ModeInfo, true
		name := s.Name
		return m, func() tea.Msg {
			raw, err := Info(name)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: ParseInfo(raw)}
		}
	case "s":
		if s == nil {
			break
		}
		m.Loading = true
		name := s.Name
		return m, func() tea.Msg { return DoneMsg{Stop(name)} }
	case "S":
		if s == nil {
			break
		}
		m.Loading = true
		name := s.Name
		return m, func() tea.Msg { return DoneMsg{Start(name)} }
	case "r":
		if s == nil {
			break
		}
		m.Loading = true
		name := s.Name
		return m, func() tea.Msg { return DoneMsg{Restart(name)} }
	case "e":
		if s == nil {
			break
		}
		m.Loading = true
		name := s.Name
		return m, func() tea.Msg { return DoneMsg{Enable(name)} }
	case "d":
		if s == nil {
			break
		}
		m.Loading = true
		name := s.Name
		return m, func() tea.Msg { return DoneMsg{Disable(name)} }
	case "R":
		m.All = List()
		m.applyFilter()
	}
	m.rebuildList()
	return m, nil
}

func (m Model) Render() string {
	helpBar := m.helpBar()
	switch m.Screen {
	case ModeLogs, ModeInfo:
		title := "Logs"
		if m.Screen == ModeInfo {
			title = "Info"
		}
		body := m.subView(title)
		fill := m.WinH - strings.Count(body, "\n") - 1 - strings.Count(helpBar, "\n") - 1
		if fill < 0 {
			fill = 0
		}
		return body + strings.Repeat("\n", fill) + helpBar
	}

	searchBar := ""
	if m.Search {
		searchBar = svcBlue.Render("  / ") + svcVal.Render(m.Filter) + svcBlue.Render("█") + "\n"
	} else if m.Filter != "" {
		searchBar = svcBlue.Render("  filter: ") + svcVal.Render(m.Filter) + svcDim.Render("  (esc clear)") + "\n"
	}

	wName, wActive, wSub, wEnabled := 36, 10, 10, 10
	header := svcHdr.Render(fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
		wName, "SERVICE", wActive, "ACTIVE", wSub, "STATE", "ENABLED")) + "\n"
	div := svcBorder.Render("  "+strings.Repeat("─", wName+wActive+wSub+wEnabled+12)) + "\n"

	status := ""
	if m.Err != "" {
		status = "  " + svcRed.Render("⚠ "+m.Err)
	} else if m.Loading {
		status = "  " + svcDim.Render("working...")
	}
	countBar := svcVal.Render(fmt.Sprintf("  %d / %d services%s", len(m.Filtered), len(m.All), status))
	helpLines := strings.Count(helpBar, "\n") + 1
	fixedLines := strings.Count(searchBar, "\n") + 2 + 1 + helpLines
	m.ListVP.Height = m.WinH - fixedLines
	if m.ListVP.Height < 1 {
		m.ListVP.Height = 1
	}
	return searchBar + header + div + m.ListVP.View() + "\n" + countBar + "\n" + helpBar
}

func (m Model) subView(title string) string {
	p := max(0, 44-len(title))
	header := svcHdr.Render("── " + title + " " + strings.Repeat("─", p))
	pct := svcDim.Render(fmt.Sprintf(" %d%%", m.scrollPct()))
	var body string
	if m.Loading {
		body = svcDim.Render("  Loading...")
	} else if m.Err != "" {
		body = svcRed.Render("  Error: " + m.Err)
	} else {
		body = m.VP.View()
	}
	return header + "\n" + body + "\n" + svcHelp.Render("  ↑/↓/PgUp/PgDn scroll · esc back") + pct
}

func (m Model) scrollPct() int {
	total := m.VP.TotalLineCount()
	if total == 0 || total <= m.VP.Height {
		return 100
	}
	return int(float64(m.VP.YOffset) / float64(total-m.VP.Height) * 100)
}

func (m Model) helpBar() string {
	help := [][]string{
		{"l", "logs"}, {"i", "info"},
		{"s", "stop"}, {"S", "start"}, {"r", "restart"},
		{"e", "enable"}, {"d", "disable"},
		{"/", "search"}, {"R", "refresh"}, {"q", "quit"},
	}
	var parts []string
	for _, h := range help {
		parts = append(parts, svcYellow.Render(h[0])+" "+svcHelp.Render(h[1]))
	}
	return svcBorder.Render(strings.Repeat("─", m.WinW)) + "\n" +
		"  " + strings.Join(parts, svcDim.Render("  ·  "))
}

func ParseInfo(raw string) string {
	wantKeys := []string{
		"Id", "Description", "LoadState", "ActiveState", "SubState",
		"UnitFileState", "MainPID", "ExecMainStartTimestamp",
		"MemoryCurrent", "CPUUsageNSec", "TasksCurrent", "FragmentPath", "Restart",
	}
	vals := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			vals[parts[0]] = parts[1]
		}
	}
	kv := func(k, v string) string {
		return svcDim.Render(fmt.Sprintf("  %-28s", k+":")) + svcVal.Render(v) + "\n"
	}
	s := ""
	for _, k := range wantKeys {
		v := vals[k]
		if v == "" || v == "[not set]" || v == "0" {
			continue
		}
		if k == "MemoryCurrent" {
			var n uint64
			fmt.Sscanf(v, "%d", &n)
			if n > 0 {
				v = helper.FmtBytes(int64(n))
			}
		}
		if k == "CPUUsageNSec" {
			var n uint64
			fmt.Sscanf(v, "%d", &n)
			if n > 0 {
				v = fmt.Sprintf("%.2f s", float64(n)/1e9)
			}
		}
		s += kv(k, v)
	}
	return s
}

// View implements tea.Model.
func (m Model) View() string { return m.Render() }
