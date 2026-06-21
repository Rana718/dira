package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Rana718/dira/internal/power"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1)
	activeStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(20)
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).MarginTop(1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
)

// ── Power TUI model ──────────────────────────────────────────────────────────

type powerState int

const (
	statePicker powerState = iota
	stateCustom
	stateDone
)

type customField struct {
	label string
	key   string
	value string
}

type powerModel struct {
	profiles []power.Profile
	cursor   int
	active   string
	sysinfo  power.SysInfo
	state    powerState
	chosen   *power.Profile

	// custom profile editor
	fields    []customField
	fieldIdx  int
	inputMode bool
	inputBuf  string
	errMsg    string
}

func newPowerModel() powerModel {
	active := power.GetActiveProfile()
	profiles := append([]power.Profile{}, power.BuiltinProfiles...)
	custom, _ := power.LoadCustomProfiles()
	profiles = append(profiles, custom...)
	// add "custom" option at end
	profiles = append(profiles, power.Profile{Name: "+ new custom profile"})

	cursor := 0
	for i, p := range profiles {
		if p.Name == active {
			cursor = i
			break
		}
	}

	return powerModel{
		profiles: profiles,
		cursor:   cursor,
		active:   active,
		sysinfo:  power.ReadSysInfo(),
	}
}

func (m powerModel) Init() tea.Cmd { return nil }

func (m powerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case statePicker:
		return m.updatePicker(msg)
	case stateCustom:
		return m.updateCustom(msg)
	}
	return m, nil
}

func (m powerModel) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}
	case "enter", " ":
		p := m.profiles[m.cursor]
		if p.Name == "+ new custom profile" {
			m.state = stateCustom
			m.fields = defaultCustomFields()
			m.fieldIdx = 0
			return m, nil
		}
		m.chosen = &p
		m.state = stateDone
		return m, tea.Quit
	case "ctrl+c", "q":
		m.state = stateDone
		return m, tea.Quit
	}
	return m, nil
}

func (m powerModel) updateCustom(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.inputMode {
		switch k.String() {
		case "enter":
			m.fields[m.fieldIdx].value = m.inputBuf
			m.inputBuf = ""
			m.inputMode = false
			if m.fieldIdx < len(m.fields)-1 {
				m.fieldIdx++
			}
		case "backspace", "ctrl+h":
			if len(m.inputBuf) > 0 {
				m.inputBuf = m.inputBuf[:len(m.inputBuf)-1]
			}
		case "ctrl+c", "esc":
			m.inputMode = false
			m.inputBuf = ""
		default:
			if len(k.String()) == 1 {
				m.inputBuf += k.String()
			}
		}
		return m, nil
	}

	switch k.String() {
	case "up", "k":
		if m.fieldIdx > 0 {
			m.fieldIdx--
		}
	case "down", "j":
		if m.fieldIdx < len(m.fields)-1 {
			m.fieldIdx++
		}
	case "enter", " ":
		m.inputMode = true
		m.inputBuf = m.fields[m.fieldIdx].value
	case "s":
		p, err := buildProfile(m.fields)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.chosen = &p
		m.state = stateDone
		return m, tea.Quit
	case "ctrl+c", "q":
		m.state = stateDone
		return m, tea.Quit
	}
	return m, nil
}

func (m powerModel) View() string {
	switch m.state {
	case statePicker:
		return m.viewPicker()
	case stateCustom:
		return m.viewCustom()
	}
	return ""
}

func (m powerModel) viewPicker() string {
	s := headerStyle.Render("Power Profile") + "\n\n"

	for i, p := range m.profiles {
		cursor := "  "
		style := normStyle
		if i == m.cursor {
			cursor = "▶ "
			style = activeStyle
		}
		suffix := ""
		if p.Name == m.active {
			suffix = dimStyle.Render("  ← active")
		}
		s += style.Render(cursor+p.Name) + suffix + "\n"
	}

	// profile config for hovered profile
	hovered := m.profiles[m.cursor]
	si := m.sysinfo

	s += sectionStyle.Render("\n─── Profile Config ───────────────") + "\n"
	if hovered.Name == "+ new custom profile" {
		s += dimStyle.Render("  Create a new custom profile\n")
	} else {
		cpuMax := "no limit"
		if hovered.CPUMaxMHz > 0 {
			cpuMax = fmt.Sprintf("%d MHz", hovered.CPUMaxMHz)
		}
		gpuMax := "no limit"
		if hovered.GPUMaxMHz > 0 {
			gpuMax = fmt.Sprintf("%d MHz", hovered.GPUMaxMHz)
		}
		s += row("CPU max clock", cpuMax)
		s += row("CPU PL1 (long)", fmt.Sprintf("%d W", hovered.PL1Watts))
		s += row("CPU PL2 (short)", fmt.Sprintf("%d W", hovered.PL2Watts))
		s += row("GPU max clock", gpuMax)
		s += row("GPU power limit", fmt.Sprintf("%d W", hovered.GPUWatts))
		s += row("GNOME policy", hovered.GNOMEPolicy)
	}

	// live system info
	s += sectionStyle.Render("─── Live System ──────────────────") + "\n"
	s += row("CPU clock", fmt.Sprintf("%d MHz  (hw max %d MHz)", si.CPUCurMHz, si.CPUMaxMHz))
	s += row("CPU TDP", fmt.Sprintf("PL1 %dW  PL2 %dW", si.PL1Watts, si.PL2Watts))
	s += row("CPU temp", fmt.Sprintf("%d°C", si.CPUTempPkg))
	s += row("CPU fan", fmt.Sprintf("%d RPM", si.CPUFanRPM))
	if si.GPUName != "" {
		s += sectionStyle.Render("─── GPU ──────────────────────────") + "\n"
		s += row("GPU", si.GPUName)
		s += row("GPU clock", fmt.Sprintf("%d MHz  (hw max %d MHz)", si.GPUCurMHz, si.GPUMaxMHz))
		s += row("GPU power", fmt.Sprintf("%.1f W  (max %.1f W)", si.GPUWatts, si.GPUMaxWatts))
		s += row("GPU temp", fmt.Sprintf("%d°C", si.GPUTemp))
		s += row("GPU fan", fmt.Sprintf("%d RPM", si.GPUFanRPM))
	}

	s += dimStyle.Render("\n↑/↓ navigate · enter apply · q quit")
	return s
}

func (m powerModel) viewCustom() string {
	s := headerStyle.Render("Custom Profile") + "\n\n"
	for i, f := range m.fields {
		val := f.value
		if i == m.fieldIdx {
			if m.inputMode {
				val = m.inputBuf + cursorStyle.Render("█")
			}
			s += activeStyle.Render("▶ ") + labelStyle.Render(f.label) + valueStyle.Render(val) + "\n"
		} else {
			s += "  " + labelStyle.Render(f.label) + dimStyle.Render(val) + "\n"
		}
	}
	if m.errMsg != "" {
		s += "\n" + errorStyle.Render("Error: "+m.errMsg)
	}
	s += dimStyle.Render("\n↑/↓ navigate · enter edit · s save & apply · q cancel")
	return s
}

func row(label, val string) string {
	return labelStyle.Render("  "+label+":") + valueStyle.Render(val) + "\n"
}

func defaultCustomFields() []customField {
	return []customField{
		{label: "Profile name", key: "name", value: "my-profile"},
		{label: "CPU max MHz", key: "cpu_max_mhz", value: "4500"},
		{label: "PL1 watts (long)", key: "pl1", value: "35"},
		{label: "PL2 watts (short)", key: "pl2", value: "45"},
		{label: "GPU max MHz (0=off)", key: "gpu_max_mhz", value: "0"},
		{label: "GPU watts (0=off)", key: "gpu_watts", value: "60"},
		{label: "GNOME policy", key: "gnome", value: "balanced"},
	}
}

func buildProfile(fields []customField) (power.Profile, error) {
	m := map[string]string{}
	for _, f := range fields {
		m[f.key] = strings.TrimSpace(f.value)
	}
	if m["name"] == "" {
		return power.Profile{}, fmt.Errorf("name required")
	}
	parseInt := func(k string) (int, error) { return strconv.Atoi(m[k]) }
	cpu, err := parseInt("cpu_max_mhz")
	if err != nil {
		return power.Profile{}, fmt.Errorf("invalid cpu_max_mhz")
	}
	pl1, err := parseInt("pl1")
	if err != nil {
		return power.Profile{}, fmt.Errorf("invalid pl1")
	}
	pl2, err := parseInt("pl2")
	if err != nil {
		return power.Profile{}, fmt.Errorf("invalid pl2")
	}
	gpu, err := parseInt("gpu_max_mhz")
	if err != nil {
		return power.Profile{}, fmt.Errorf("invalid gpu_max_mhz")
	}
	gpuW, err := parseInt("gpu_watts")
	if err != nil {
		return power.Profile{}, fmt.Errorf("invalid gpu_watts")
	}
	return power.Profile{
		Name:        m["name"],
		CPUMaxMHz:   cpu,
		PL1Watts:    pl1,
		PL2Watts:    pl2,
		GPUMaxMHz:   gpu,
		GPUWatts:    gpuW,
		GNOMEPolicy: m["gnome"],
	}, nil
}

// ── Command ──────────────────────────────────────────────────────────────────

var powerCmd = &cobra.Command{
	Use:   "power",
	Short: "Manage power profile (CPU/GPU limits, TDP)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newPowerModel()
		result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		pm := result.(powerModel)
		if pm.chosen == nil {
			return nil
		}
		p := *pm.chosen
		// save custom profile first
		if pm.state == stateDone && !isBuiltin(p.Name) {
			if err := power.SaveCustomProfile(p); err != nil {
				return err
			}
		}
		fmt.Printf("Applying profile: %s\n", p.Name)
		return power.Apply(p)
	},
}

func isBuiltin(name string) bool {
	for _, p := range power.BuiltinProfiles {
		if p.Name == name {
			return true
		}
	}
	return false
}
