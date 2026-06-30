package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Rana718/dira/internal/container"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	ctHdr    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	ctSel    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	ctDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ctGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	ctRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	ctYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	ctValue  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ctHelp   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ctDocker = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	ctPodman = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	ctBorder = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	ctCPU    = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ctMem    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	ctNet    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

// ── Messages ─────────────────────────────────────────────────────────────────

type ctView int

const (
	viewList ctView = iota
	viewLogs
	viewInfo
	viewVolumes
)

type contentMsg    struct{ text, err string }
type statsTickMsg  struct{}
type allStatsMsg   struct{ stats map[string]container.Stats }
type actionDoneMsg struct{ err error }

func statsTickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg {
		return statsTickMsg{}
	})
}

// ── Model ─────────────────────────────────────────────────────────────────────

type containerModel struct {
	containers []container.Container
	statsMap   map[string]container.Stats // id → stats
	cursor     int
	view       ctView
	vp         viewport.Model
	winW, winH int
	err        string
	loading    bool
}

func newContainerModel() containerModel {
	vp := viewport.New(80, 20)
	return containerModel{
		containers: container.List(),
		statsMap:   map[string]container.Stats{},
		vp:         vp,
	}
}

func (m containerModel) Init() tea.Cmd {
	return statsTickCmd()
}

func (m containerModel) selected() *container.Container {
	if len(m.containers) == 0 || m.cursor >= len(m.containers) {
		return nil
	}
	c := m.containers[m.cursor]
	return &c
}

func (m containerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.winW = msg.Width
		m.winH = msg.Height
		m.vp.Width = msg.Width - 2
		// header(1) + border(1) + footer(1) + helpbar(2) = 5
		m.vp.Height = msg.Height - 5
		return m, nil

	case statsTickMsg:
		// fetch stats for all running containers in background
		containers := m.containers
		return m, func() tea.Msg {
			result := map[string]container.Stats{}
			for _, c := range containers {
				if !c.Running {
					continue
				}
				s, err := container.GetStats(c.Runtime, c.ID)
				if err == nil {
					result[c.ID] = s
				}
			}
			return allStatsMsg{stats: result}
		}

	case allStatsMsg:
		m.statsMap = msg.stats
		return m, statsTickCmd()

	case contentMsg:
		m.loading = false
		m.err = msg.err
		m.vp.SetContent(msg.text)
		m.vp.GotoTop()
		return m, nil

	case actionDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.containers = container.List()
			m.err = ""
		}
		return m, nil

	case tea.KeyMsg:
		if m.view != viewList {
			return m.updateSubView(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m containerModel) updateSubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "backspace":
		m.view = viewList
		m.err = ""
		m.loading = false
		m.vp.SetContent("")
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m containerModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.selected()
	m.err = ""

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.containers)-1 {
			m.cursor++
		}
	case "r":
		m.containers = container.List()
	case "q", "ctrl+c":
		return m, tea.Quit

	case "l":
		if c == nil {
			break
		}
		m.view = viewLogs
		m.loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			raw, err := container.Logs(rt, id, 200)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			return contentMsg{text: colorizeLogs(raw), err: errStr}
		}

	case "i":
		if c == nil {
			break
		}
		m.view = viewInfo
		m.loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			info, err := container.Inspect(rt, id)
			if err != nil {
				return contentMsg{err: err.Error()}
			}
			return contentMsg{text: renderInfo(info)}
		}

	case "v":
		if c == nil {
			break
		}
		m.view = viewVolumes
		m.loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			info, err := container.Inspect(rt, id)
			if err != nil {
				return contentMsg{err: err.Error()}
			}
			return contentMsg{text: renderVolumes(info)}
		}

	case "s":
		if c == nil || !c.Running {
			m.err = "container is not running"
			break
		}
		m.loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			return actionDoneMsg{err: container.Stop(rt, id)}
		}

	case "S":
		if c == nil || c.Running {
			m.err = "container is already running"
			break
		}
		m.loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			return actionDoneMsg{err: container.Start(rt, id)}
		}

	case "e":
		if c == nil || !c.Running {
			m.err = "container is not running"
			break
		}
		return m, tea.ExecProcess(
			exec.Command(c.Runtime, "exec", "-it", c.ID, "/bin/sh"),
			func(err error) tea.Msg { return actionDoneMsg{err: err} },
		)
	}
	return m, nil
}

// ── Views ─────────────────────────────────────────────────────────────────────

func (m containerModel) View() string {
	helpBar := m.renderHelp()

	var body string
	switch m.view {
	case viewLogs:
		body = m.subView("Logs", "  ↑/↓/PgUp/PgDn scroll · esc back")
	case viewInfo:
		body = m.subView("Info", "  ↑/↓ scroll · esc back")
	case viewVolumes:
		body = m.subView("Volumes / Mounts", "  ↑/↓ scroll · esc back")
	default:
		body = m.viewList()
	}

	// fill remaining lines so helpBar is always at the very bottom
	bodyLines := strings.Count(body, "\n") + 1
	helpLines := strings.Count(helpBar, "\n") + 1
	fill := m.winH - bodyLines - helpLines
	if fill < 0 {
		fill = 0
	}
	return body + strings.Repeat("\n", fill) + helpBar
}

func (m containerModel) renderHelp() string {
	help := [][]string{
		{"l", "logs"}, {"i", "info"}, {"v", "volumes"},
		{"s", "stop"}, {"S", "start"}, {"e", "shell"},
		{"r", "refresh"}, {"q", "quit"},
	}
	var parts []string
	for _, h := range help {
		parts = append(parts, ctYellow.Render(h[0])+" "+ctHelp.Render(h[1]))
	}
	bar := "  " + strings.Join(parts, ctDim.Render("  ·  "))
	return ctBorder.Render(strings.Repeat("─", m.winW)) + "\n" + bar
}

func (m containerModel) subView(title, helpText string) string {
	header := ctHdr.Render("── "+title+" "+strings.Repeat("─", max(0, 44-len(title))))
	scroll := ctDim.Render(fmt.Sprintf(" %d%%", m.scrollPct()))

	var body string
	if m.loading {
		body = ctDim.Render("  Loading...")
	} else if m.err != "" {
		body = ctRed.Render("  Error: " + m.err)
	} else {
		body = m.vp.View()
	}

	footer := ctHelp.Render(helpText) + scroll
	return header + "\n" + body + "\n" + footer
}

func (m containerModel) scrollPct() int {
	total := m.vp.TotalLineCount()
	if total == 0 || total <= m.vp.Height {
		return 100
	}
	return int(float64(m.vp.YOffset) / float64(total-m.vp.Height) * 100)
}

func (m containerModel) viewList() string {
	// column widths
	wName, wID, wImage, wStatus, wRuntime := 18, 12, 38, 11, 8

	// header row
	s := ctHdr.Render(
		"  "+pad("NAME", wName)+"  "+pad("ID", wID)+"  "+pad("RUNTIME", wRuntime)+"  "+pad("STATUS", wStatus)+"  "+
			pad("IMAGE", wImage)+"  "+"CPU       MEM",
	) + "\n"
	s += ctBorder.Render("  "+strings.Repeat("─", wName+wID+wRuntime+wStatus+wImage+32)) + "\n"

	for i, c := range m.containers {
		cursor := "  "
		nameStyle := ctDim
		if i == m.cursor {
			cursor = "▶ "
			nameStyle = ctSel
		}

		rtLabel := ctDocker.Render(pad("[docker]", wRuntime))
		if c.Runtime == "podman" {
			rtLabel = ctPodman.Render(pad("[podman]", wRuntime))
		}

		status := ctRed.Render(pad("● stopped", wStatus))
		if c.Running {
			status = ctGreen.Render(pad("● running", wStatus))
		}

		image := c.Image
		if len(image) > wImage {
			image = image[:wImage-3] + "..."
		}

		cpu, mem := ctDim.Render("—"), ctDim.Render("—")
		if st, ok := m.statsMap[c.ID]; ok && c.Running {
			cpu = ctCPU.Render(pad(st.CPU, 9))
			memParts := strings.SplitN(st.MemUsage, " / ", 2)
			mem = ctMem.Render(memParts[0])
		}

		s += fmt.Sprintf("%s%s  %s  %s  %s  %s  %s  %s\n",
			cursor,
			nameStyle.Render(pad(c.Name, wName)),
			ctDim.Render(pad(c.ID, wID)),
			rtLabel,
			status,
			ctDim.Render(pad(image, wImage)),
			cpu,
			mem,
		)
	}

	if len(m.containers) == 0 {
		s += ctDim.Render("  No containers found (docker / podman)") + "\n"
	}
	if m.err != "" {
		s += "\n" + ctRed.Render("  ⚠ "+m.err) + "\n"
	}
	return s
}

// ── Renderers ────────────────────────────────────────────────────────────────

func colorizeLogs(raw string) string {
	var sb strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic"):
			sb.WriteString(ctRed.Render(line))
		case strings.Contains(lower, "warn"):
			sb.WriteString(ctYellow.Render(line))
		case strings.Contains(lower, "info"):
			sb.WriteString(ctGreen.Render(line))
		default:
			sb.WriteString(ctValue.Render(line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderInfo(info container.InspectInfo) string {
	kv := func(k, v string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(16).Render("  "+k+":") +
			ctValue.Render(v) + "\n"
	}
	s := kv("ID", info.ID)
	s += kv("Name", info.Name)
	s += kv("Image", info.Image)
	s += kv("Status", info.Status)
	s += kv("Created", info.Created)

	// resource limits
	r := info.Resources
	s += "\n" + ctHdr.Render("  Resource Limits:") + "\n"
	if r.MemoryBytes > 0 {
		s += kv("Memory limit", fmt.Sprintf("%s", fmtBytes(r.MemoryBytes)))
	} else {
		s += kv("Memory limit", ctDim.Render("unlimited"))
	}
	if r.MemorySwapBytes > 0 && r.MemorySwapBytes != r.MemoryBytes {
		s += kv("Swap limit", fmtBytes(r.MemorySwapBytes))
	} else if r.MemorySwapBytes == r.MemoryBytes && r.MemoryBytes > 0 {
		s += kv("Swap limit", ctYellow.Render("disabled (swap = memory)"))
	}
	if r.NanoCPUs > 0 {
		s += kv("CPU limit", fmt.Sprintf("%.2f CPUs", float64(r.NanoCPUs)/1e9))
	} else {
		s += kv("CPU limit", ctDim.Render("unlimited"))
	}
	if r.CPUShares > 0 {
		s += kv("CPU shares", fmt.Sprintf("%d", r.CPUShares))
	}
	if r.PidsLimit > 0 {
		s += kv("PID limit", fmt.Sprintf("%d", r.PidsLimit))
	} else {
		s += kv("PID limit", ctDim.Render("unlimited"))
	}

	if len(info.Mounts) > 0 {
		s += "\n" + ctHdr.Render("  Mounts:") + "\n"
		for _, m := range info.Mounts {
			s += fmt.Sprintf("    %s  →  %s\n", ctDim.Render(m.Source), ctValue.Render(m.Destination))
		}
	}
	if len(info.Env) > 0 {
		s += "\n" + ctHdr.Render("  Environment:") + "\n"
		for _, e := range info.Env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				s += fmt.Sprintf("    %s = %s\n", ctYellow.Render(parts[0]), ctValue.Render(parts[1]))
			} else {
				s += "    " + e + "\n"
			}
		}
	}
	return s
}

func fmtBytes(b int64) string {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.0f MB", float64(b)/MB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func renderVolumes(info container.InspectInfo) string {
	if len(info.Mounts) == 0 {
		return ctDim.Render("  No volumes or mounts found.")
	}
	s := ""
	for i, m := range info.Mounts {
		s += ctHdr.Render(fmt.Sprintf("  Mount %d", i+1)) + "\n"
		s += fmt.Sprintf("    %-16s %s\n", ctDim.Render("Source:"), ctValue.Render(m.Source))
		s += fmt.Sprintf("    %-16s %s\n", ctDim.Render("Destination:"), ctValue.Render(m.Destination))
		if m.Mode != "" {
			s += fmt.Sprintf("    %-16s %s\n", ctDim.Render("Mode:"), ctValue.Render(m.Mode))
		}
		s += "\n"
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── Command ──────────────────────────────────────────────────────────────────

var containerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage Docker and Podman containers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newContainerModel()
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}
