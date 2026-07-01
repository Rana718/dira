package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Rana718/dira/internal/service"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── Styles ────────────────────────────────────────────────────────────────────

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

// ── Messages ──────────────────────────────────────────────────────────────────

type svcView int

const (
	svcViewList svcView = iota
	svcViewLogs
	svcViewInfo
)

type svcContentMsg struct{ text, err string }
type svcDoneMsg    struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

type serviceModel struct {
	all      []service.Entry
	filtered []service.Entry
	cursor   int
	view     svcView
	vp       viewport.Model // for logs/info sub-views
	listVP   viewport.Model // for the main list
	winW     int
	winH     int
	filter   string
	search   bool
	loading  bool
	err      string
}

func newServiceModel() serviceModel {
	svcs := service.List()
	return serviceModel{
		all:      svcs,
		filtered: svcs,
		vp:       viewport.New(80, 20),
		listVP:   viewport.New(80, 20),
	}
}

func (m *serviceModel) applyFilter() {
	if m.filter == "" {
		m.filtered = m.all
	} else {
		f := strings.ToLower(m.filter)
		var out []service.Entry
		for _, s := range m.all {
			if strings.Contains(strings.ToLower(s.Name), f) ||
				strings.Contains(strings.ToLower(s.Description), f) {
				out = append(out, s)
			}
		}
		m.filtered = out
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
	m.rebuildListContent()
}

func (m *serviceModel) selected() *service.Entry {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	s := m.filtered[m.cursor]
	return &s
}

func (m serviceModel) Init() tea.Cmd { return nil }

func (m serviceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.winW = msg.Width
		m.winH = msg.Height
		m.vp.Width = msg.Width - 2
		m.vp.Height = msg.Height - 5
		m.listVP.Width = msg.Width - 2
		m.listVP.Height = msg.Height - 8
		m.rebuildListContent()
		return m, nil

	case svcContentMsg:
		m.loading = false
		m.err = msg.err
		m.vp.SetContent(msg.text)
		m.vp.GotoTop()
		return m, nil

	case svcDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
			m.all = service.List()
			m.applyFilter()
		}
		m.rebuildListContent()
		return m, nil

	case tea.KeyMsg:
		if m.view != svcViewList {
			return m.updateSubView(msg)
		}
		if m.search {
			return m.updateSearch(msg)
		}
		// PgUp/PgDn/g/G scroll the list viewport directly
		switch msg.String() {
		case "pgup", "ctrl+u":
			m.listVP, _ = m.listVP.Update(msg)
			return m, nil
		case "pgdn", "ctrl+d":
			m.listVP, _ = m.listVP.Update(msg)
			return m, nil
		}
		return m.updateList(msg)

	case tea.MouseMsg:
		if m.view == svcViewList {
			var cmd tea.Cmd
			m.listVP, cmd = m.listVP.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m serviceModel) updateSubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.view = svcViewList
		m.err = ""
		m.loading = false
		m.vp.SetContent("")
		return m, nil
	}
	// pass all other keys to viewport for scrolling
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m serviceModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.search = false
	case "backspace", "ctrl+h":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.applyFilter()
		}
	default:
		if len(msg.String()) == 1 {
			m.filter += msg.String()
			m.applyFilter()
		}
	}
	m.rebuildListContent()
	return m, nil
}

func (m serviceModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.selected()
	m.err = ""

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
	case "/":
		m.search = true
	case "esc":
		m.filter = ""
		m.applyFilter()
	case "q", "ctrl+c":
		return m, tea.Quit

	case "l":
		if s == nil {
			break
		}
		m.view = svcViewLogs
		m.loading = true
		name := s.Name
		return m, func() tea.Msg {
			raw, err := service.Logs(name, 200)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			return svcContentMsg{text: colorizeLogs(raw), err: errStr}
		}

	case "i":
		if s == nil {
			break
		}
		m.view = svcViewInfo
		m.loading = true
		name := s.Name
		return m, func() tea.Msg {
			raw, err := service.Info(name)
			if err != nil {
				return svcContentMsg{err: err.Error()}
			}
			return svcContentMsg{text: parseSvcInfo(raw)}
		}

	case "s":
		if s == nil {
			break
		}
		name := s.Name
		m.loading = true
		return m, func() tea.Msg { return svcDoneMsg{service.Stop(name)} }

	case "S":
		if s == nil {
			break
		}
		name := s.Name
		m.loading = true
		return m, func() tea.Msg { return svcDoneMsg{service.Start(name)} }

	case "r":
		if s == nil {
			break
		}
		name := s.Name
		m.loading = true
		return m, func() tea.Msg { return svcDoneMsg{service.Restart(name)} }

	case "e":
		if s == nil {
			break
		}
		name := s.Name
		m.loading = true
		return m, func() tea.Msg { return svcDoneMsg{service.Enable(name)} }

	case "d":
		if s == nil {
			break
		}
		name := s.Name
		m.loading = true
		return m, func() tea.Msg { return svcDoneMsg{service.Disable(name)} }

	case "R":
		m.all = service.List()
		m.applyFilter()
		m.rebuildListContent()
	}
	m.rebuildListContent()
	return m, nil
}

// rebuildListContent renders all rows into listVP so viewport can scroll them
func (m *serviceModel) rebuildListContent() {
	wName, wActive, wSub, wEnabled := 36, 10, 10, 10
	var sb strings.Builder
	for i, svc := range m.filtered {
		cursor := "  "
		nameStyle := svcDim
		if i == m.cursor {
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
			nameStyle.Render(padR(name, wName)),
			activeStyle.Render(padR(svc.Active, wActive)),
			subStyle.Render(padR(svc.Sub, wSub)),
			enabledStyle.Render(padR(svc.Enabled, wEnabled)),
		))
	}
	if len(m.filtered) == 0 {
		sb.WriteString(svcDim.Render("  No services found") + "\n")
	}
	m.listVP.SetContent(sb.String())
}

// ensureCursorVisible scrolls listVP so cursor row is always in view
func (m *serviceModel) ensureCursorVisible() {
	if m.listVP.Height == 0 {
		return
	}
	if m.cursor < m.listVP.YOffset {
		m.listVP.SetYOffset(m.cursor)
	} else if m.cursor >= m.listVP.YOffset+m.listVP.Height {
		m.listVP.SetYOffset(m.cursor - m.listVP.Height + 1)
	}
}

// ── Views ─────────────────────────────────────────────────────────────────────

func (m serviceModel) View() string {
	helpBar := m.svcHelpBar()

	switch m.view {
	case svcViewLogs:
		body := m.svcSubView("Logs")
		fill := m.winH - strings.Count(body, "\n") - 1 - strings.Count(helpBar, "\n") - 1
		if fill < 0 {
			fill = 0
		}
		return body + strings.Repeat("\n", fill) + helpBar
	case svcViewInfo:
		body := m.svcSubView("Info")
		fill := m.winH - strings.Count(body, "\n") - 1 - strings.Count(helpBar, "\n") - 1
		if fill < 0 {
			fill = 0
		}
		return body + strings.Repeat("\n", fill) + helpBar
	}

	// list view: fixed layout so count is always just above helpbar
	searchBar := ""
	if m.search {
		searchBar = svcBlue.Render("  / ") + svcVal.Render(m.filter) + svcBlue.Render("█") + "\n"
	} else if m.filter != "" {
		searchBar = svcBlue.Render("  filter: ") + svcVal.Render(m.filter) + svcDim.Render("  (esc clear)") + "\n"
	}

	wName, wActive, wSub, wEnabled := 36, 10, 10, 10
	header := svcHdr.Render(fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
		wName, "SERVICE", wActive, "ACTIVE", wSub, "STATE", "ENABLED")) + "\n"
	div := svcBorder.Render("  "+strings.Repeat("─", wName+wActive+wSub+wEnabled+12)) + "\n"

	status := ""
	if m.err != "" {
		status = "  " + svcRed.Render("⚠ "+m.err)
	} else if m.loading {
		status = "  " + svcDim.Render("working...")
	}
	countBar := svcVal.Render(fmt.Sprintf("  %d / %d services%s", len(m.filtered), len(m.all), status))
	helpLines := strings.Count(helpBar, "\n") + 1

	// fixed heights: searchbar(0-1) + header(1) + div(1) + count(1) + helpbar
	fixedLines := strings.Count(searchBar, "\n") + 2 + 1 + helpLines
	m.listVP.Height = m.winH - fixedLines
	if m.listVP.Height < 1 {
		m.listVP.Height = 1
	}

	return searchBar + header + div + m.listVP.View() + "\n" + countBar + "\n" + helpBar
}

func (m serviceModel) svcSubView(title string) string {
	pad := max(0, 44-len(title))
	header := svcHdr.Render("── " + title + " " + strings.Repeat("─", pad))
	pct := svcDim.Render(fmt.Sprintf(" %d%%", m.svcScrollPct()))
	var body string
	if m.loading {
		body = svcDim.Render("  Loading...")
	} else if m.err != "" {
		body = svcRed.Render("  Error: " + m.err)
	} else {
		body = m.vp.View()
	}
	return header + "\n" + body + "\n" + svcHelp.Render("  ↑/↓/PgUp/PgDn scroll · esc back") + pct
}

func (m serviceModel) svcScrollPct() int {
	total := m.vp.TotalLineCount()
	if total == 0 || total <= m.vp.Height {
		return 100
	}
	return int(float64(m.vp.YOffset) / float64(total-m.vp.Height) * 100)
}

func (m serviceModel) svcListView() string {
	wName, wActive, wSub, wEnabled := 36, 10, 10, 10

	searchBar := ""
	if m.search {
		searchBar = svcBlue.Render("  / ") + svcVal.Render(m.filter) + svcBlue.Render("█") + "\n"
	} else if m.filter != "" {
		searchBar = svcBlue.Render("  filter: ") + svcVal.Render(m.filter) + svcDim.Render("  (esc clear)") + "\n"
	}

	header := svcHdr.Render(fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
		wName, "SERVICE", wActive, "ACTIVE", wSub, "STATE", "ENABLED")) + "\n"
	div := svcBorder.Render("  "+strings.Repeat("─", wName+wActive+wSub+wEnabled+12)) + "\n"

	status := ""
	if m.err != "" {
		status = "  " + svcRed.Render("⚠ "+m.err)
	} else if m.loading {
		status = "  " + svcDim.Render("working...")
	}

	count := svcDim.Render(fmt.Sprintf("  %d / %d services%s",
		len(m.filtered), len(m.all), status))

	return searchBar + header + div + m.listVP.View() + "\n" + count
}

func (m serviceModel) svcHelpBar() string {
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
	return svcBorder.Render(strings.Repeat("─", m.winW)) + "\n" +
		"  " + strings.Join(parts, svcDim.Render("  ·  "))
}

// padR pads with spaces based on rune count (handles multi-byte chars)
func padR(s string, width int) string {
	r := len([]rune(s))
	if r >= width {
		return s
	}
	return s + strings.Repeat(" ", width-r)
}

func parseSvcInfo(raw string) string {
	wantKeys := []string{
		"Id", "Description", "LoadState", "ActiveState", "SubState",
		"UnitFileState", "MainPID", "ExecMainStartTimestamp",
		"MemoryCurrent", "CPUUsageNSec", "TasksCurrent",
		"FragmentPath", "Restart",
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
				v = fmtBytes(int64(n))
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

// ── Command ───────────────────────────────────────────────────────────────────

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage systemd services",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newServiceModel()
		if _, err := tea.NewProgram(m,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}
