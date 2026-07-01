package container

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Rana718/dira/internal/helper"
	"github.com/Rana718/dira/internal/tui"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

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

// ── Messages ──────────────────────────────────────────────────────────────────

type Mode int

const (
	ModeList Mode = iota
	ModeLogs
	ModeInfo
	ModeVolumes
)

type ContentMsg    struct{ Text, Err string }
type StatsTickMsg  struct{}
type AllStatsMsg   struct{ Stats map[string]Stats }
type ActionDoneMsg struct{ Err error }

func StatsTickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg {
		return StatsTickMsg{}
	})
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	Containers []Container
	StatsMap   map[string]Stats
	Cursor     int
	Screen Mode
	VP         viewport.Model
	WinW, WinH int
	Err        string
	Loading    bool
}

func NewModel() Model {
	return Model{
		Containers: List(),
		StatsMap:   map[string]Stats{},
		VP:         viewport.New(80, 20),
	}
}

func (m Model) Init() tea.Cmd { return StatsTickCmd() }

func (m Model) Selected() *Container {
	if len(m.Containers) == 0 || m.Cursor >= len(m.Containers) {
		return nil
	}
	c := m.Containers[m.Cursor]
	return &c
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.WinW = msg.Width
		m.WinH = msg.Height
		m.VP.Width = msg.Width - 2
		m.VP.Height = msg.Height - 5
		return m, nil

	case StatsTickMsg:
		containers := m.Containers
		return m, func() tea.Msg {
			result := map[string]Stats{}
			for _, c := range containers {
				if !c.Running {
					continue
				}
				s, err := GetStats(c.Runtime, c.ID)
				if err == nil {
					result[c.ID] = s
				}
			}
			return AllStatsMsg{Stats: result}
		}

	case AllStatsMsg:
		m.StatsMap = msg.Stats
		return m, StatsTickCmd()

	case ContentMsg:
		m.Loading = false
		m.Err = msg.Err
		m.VP.SetContent(msg.Text)
		m.VP.GotoTop()
		return m, nil

	case ActionDoneMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err.Error()
		} else {
			m.Containers = List()
			m.Err = ""
		}
		return m, nil

	case tea.KeyMsg:
		if m.Screen != ModeList {
			return m.updateSubView(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m Model) updateSubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "backspace":
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

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.Selected()
	m.Err = ""
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Containers)-1 {
			m.Cursor++
		}
	case "r":
		m.Containers = List()
	case "q", "ctrl+c":
		return m, tea.Quit
	case "l":
		if c == nil {
			break
		}
		m.Screen = ModeLogs
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			raw, err := Logs(rt, id, 200)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			return ContentMsg{Text: tui.ColorizeLogs(raw), Err: errStr}
		}
	case "i":
		if c == nil {
			break
		}
		m.Screen = ModeInfo
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			info, err := Inspect(rt, id)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: RenderInfo(info)}
		}
	case "v":
		if c == nil {
			break
		}
		m.Screen = ModeVolumes
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			info, err := Inspect(rt, id)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: RenderVolumes(info)}
		}
	case "d":
		if c == nil {
			break
		}
		if c.Running {
			m.Err = "stop the container first before deleting"
			break
		}
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: Delete(rt, id)} }
	case "s":
		if c == nil || !c.Running {
			m.Err = "container is not running"
			break
		}
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: Stop(rt, id)} }
	case "S":
		if c == nil || c.Running {
			m.Err = "container is already running"
			break
		}
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: Start(rt, id)} }
	case "e":
		if c == nil || !c.Running {
			m.Err = "container is not running"
			break
		}
		return m, tea.ExecProcess(
			exec.Command(c.Runtime, "exec", "-it", c.ID, "/bin/sh"),
			func(err error) tea.Msg { return ActionDoneMsg{Err: err} },
		)
	}
	return m, nil
}

// ── Views ─────────────────────────────────────────────────────────────────────

func (m Model) Render() string {
	helpBar := m.renderHelp()
	var body string
	switch m.Screen {
	case ModeLogs:
		body = m.subView("Logs", "  ↑/↓/PgUp/PgDn scroll · esc back")
	case ModeInfo:
		body = m.subView("Info", "  ↑/↓ scroll · esc back")
	case ModeVolumes:
		body = m.subView("Volumes / Mounts", "  ↑/↓ scroll · esc back")
	default:
		body = m.viewList()
	}
	fill := m.WinH - strings.Count(body, "\n") - 1 - strings.Count(helpBar, "\n") - 1
	if fill < 0 {
		fill = 0
	}
	return body + strings.Repeat("\n", fill) + helpBar
}

func (m Model) renderHelp() string {
	help := [][]string{
		{"l", "logs"}, {"i", "info"}, {"v", "volumes"},
		{"s", "stop"}, {"S", "start"}, {"d", "delete"},
		{"e", "shell"}, {"r", "refresh"}, {"q", "quit"},
	}
	var parts []string
	for _, h := range help {
		parts = append(parts, ctYellow.Render(h[0])+" "+ctHelp.Render(h[1]))
	}
	return ctBorder.Render(strings.Repeat("─", m.WinW)) + "\n" +
		"  " + strings.Join(parts, ctDim.Render("  ·  "))
}

func (m Model) subView(title, helpText string) string {
	header := ctHdr.Render("── " + title + " " + strings.Repeat("─", helper.MaxInt(0, 44-len(title))))
	scroll := ctDim.Render(fmt.Sprintf(" %d%%", m.scrollPct()))
	var body string
	if m.Loading {
		body = ctDim.Render("  Loading...")
	} else if m.Err != "" {
		body = ctRed.Render("  Error: " + m.Err)
	} else {
		body = m.VP.View()
	}
	return header + "\n" + body + "\n" + ctHelp.Render(helpText) + scroll
}

func (m Model) scrollPct() int {
	total := m.VP.TotalLineCount()
	if total == 0 || total <= m.VP.Height {
		return 100
	}
	return int(float64(m.VP.YOffset) / float64(total-m.VP.Height) * 100)
}

func (m Model) viewList() string {
	wName, wID, wRuntime, wStatus, wUptime, wPorts := 18, 12, 8, 9, 16, 14
	s := ctHdr.Render(
		"  "+helper.Pad("NAME", wName)+"  "+helper.Pad("ID", wID)+"  "+helper.Pad("RUNTIME", wRuntime)+
			"  "+helper.Pad("STATUS", wStatus)+"  "+helper.Pad("UPTIME", wUptime)+
			"  "+helper.Pad("PORTS", wPorts)+"  CPU        MEM",
	) + "\n"
	s += ctBorder.Render("  "+strings.Repeat("─", wName+wID+wRuntime+wStatus+wUptime+wPorts+36)) + "\n"

	for i, c := range m.Containers {
		cursor := "  "
		nameStyle := ctDim
		if i == m.Cursor {
			cursor = "▶ "
			nameStyle = ctSel
		}
		rtLabel := ctDocker.Render(helper.Pad("[docker]", wRuntime))
		if c.Runtime == "podman" {
			rtLabel = ctPodman.Render(helper.Pad("[podman]", wRuntime))
		}
		statusText := helper.Pad("  stopped", wStatus)
		status := ctRed.Render(statusText)
		if c.Running {
			status = ctGreen.Render(helper.Pad("  running", wStatus))
		}
		uptimeStr := strings.TrimSuffix(c.RunningFor, " ago")
		if uptimeStr == "" {
			uptimeStr = " "
		}
		if len([]rune(uptimeStr)) > wUptime {
			uptimeStr = string([]rune(uptimeStr)[:wUptime-1]) + "…"
		}
		uptimeStr = helper.PadR(uptimeStr, wUptime)

		portsStr := "—"
		if c.Ports != "" {
			portsStr = c.Ports
		}
		if len([]rune(portsStr)) > wPorts {
			portsStr = string([]rune(portsStr)[:wPorts-1]) + "…"
		}
		portsStr = helper.PadR(portsStr, wPorts)
		portColor := ctDim
		if c.Ports != "" {
			portColor = ctNet
		}

		cpuStr := strings.Repeat(" ", 9)
		memStr := strings.Repeat(" ", 10)
		cpuColor, memColor := ctDim, ctDim
		if st, ok := m.StatsMap[c.ID]; ok && c.Running {
			cpuStr = helper.Pad(st.CPU, 9)
			cpuColor = ctCPU
			memParts := strings.SplitN(st.MemUsage, " / ", 2)
			memStr = helper.Pad(memParts[0], 10)
			memColor = ctMem
		}
		s += fmt.Sprintf("%s%s  %s  %s  %s  %s  %s  %s  %s\n",
			cursor,
			nameStyle.Render(helper.Pad(c.Name, wName)),
			ctDim.Render(helper.Pad(c.ID, wID)),
			rtLabel, status,
			ctValue.Render(uptimeStr),
			portColor.Render(portsStr),
			cpuColor.Render(cpuStr),
			memColor.Render(memStr),
		)
	}
	if len(m.Containers) == 0 {
		s += ctDim.Render("  No containers found (docker / podman)") + "\n"
	}
	if m.Err != "" {
		s += "\n" + ctRed.Render("  ⚠ "+m.Err) + "\n"
	}
	return s
}

// ── Renderers ─────────────────────────────────────────────────────────────────

func RenderInfo(info InspectInfo) string {
	kv := func(k, v string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(16).Render("  "+k+":") +
			ctValue.Render(v) + "\n"
	}
	s := kv("ID", info.ID) + kv("Name", info.Name) + kv("Image", info.Image) +
		kv("Status", info.Status) + kv("Created", info.Created)
	r := info.Resources
	s += "\n" + ctHdr.Render("  Resource Limits:") + "\n"
	if r.MemoryBytes > 0 {
		s += kv("Memory limit", helper.FmtBytes(r.MemoryBytes))
	} else {
		s += kv("Memory limit", ctDim.Render("unlimited"))
	}
	if r.MemorySwapBytes > 0 && r.MemorySwapBytes != r.MemoryBytes {
		s += kv("Swap limit", helper.FmtBytes(r.MemorySwapBytes))
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

func RenderVolumes(info InspectInfo) string {
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

// View implements tea.Model.
func (m Model) View() string { return m.Render() }
