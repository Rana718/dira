package power

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Rana718/dira/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pwrHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1)
	pwrActive  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	pwrDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	pwrLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(20)
	pwrValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	pwrSection = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).MarginTop(1)
	pwrError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	pwrCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
)

type modelState int

const (
	statePicker modelState = iota
	stateCustom
	stateDone
)

type customField struct{ label, key, value string }

// TUIModel is the bubbletea model for the power command.
type TUIModel struct {
	profiles  []Profile
	cursor    int
	active    string
	sysinfo   SysInfo
	state     modelState
	Chosen    *Profile
	fields    []customField
	fieldIdx  int
	inputMode bool
	inputBuf  string
	errMsg    string
}

func NewTUIModel() TUIModel {
	active := GetActiveProfile()
	profiles := append([]Profile{}, BuiltinProfiles...)
	custom, _ := LoadCustomProfiles()
	profiles = append(profiles, custom...)
	profiles = append(profiles, Profile{Name: "+ new custom profile"})
	cursor := 0
	for i, p := range profiles {
		if p.Name == active {
			cursor = i
			break
		}
	}
	return TUIModel{
		profiles: profiles,
		cursor:   cursor,
		active:   active,
		sysinfo:  ReadSysInfo(),
	}
}

func (m TUIModel) Init() tea.Cmd { return nil }

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case statePicker:
		return m.updatePicker(msg)
	case stateCustom:
		return m.updateCustom(msg)
	}
	return m, nil
}

func (m TUIModel) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.fields = defaultFields()
			m.fieldIdx = 0
			return m, nil
		}
		m.Chosen = &p
		m.state = stateDone
		return m, tea.Quit
	case "ctrl+c", "q":
		m.state = stateDone
		return m, tea.Quit
	}
	return m, nil
}

func (m TUIModel) updateCustom(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.Chosen = &p
		m.state = stateDone
		return m, tea.Quit
	case "ctrl+c", "q":
		m.state = stateDone
		return m, tea.Quit
	}
	return m, nil
}

func (m TUIModel) View() string {
	switch m.state {
	case statePicker:
		return m.viewPicker()
	case stateCustom:
		return m.viewCustom()
	}
	return ""
}

func (m TUIModel) viewPicker() string {
	s := pwrHeader.Render("Power Profile") + "\n\n"
	for i, p := range m.profiles {
		cursor := "  "
		style := tui.Dim
		if i == m.cursor {
			cursor = "▶ "
			style = pwrActive
		}
		suffix := ""
		if p.Name == m.active {
			suffix = pwrDim.Render("  ← active")
		}
		s += style.Render(cursor+p.Name) + suffix + "\n"
	}

	hovered := m.profiles[m.cursor]
	si := m.sysinfo

	s += pwrSection.Render("\n─── Profile Config ───────────────") + "\n"
	if hovered.Name == "+ new custom profile" {
		s += pwrDim.Render("  Create a new custom profile\n")
	} else {
		cpuMax := "no limit"
		if hovered.CPUMaxMHz > 0 {
			cpuMax = fmt.Sprintf("%d MHz", hovered.CPUMaxMHz)
		}
		gpuMax := "no limit"
		if hovered.GPUMaxMHz > 0 {
			gpuMax = fmt.Sprintf("%d MHz", hovered.GPUMaxMHz)
		}
		s += pwrRow("CPU max clock", cpuMax)
		s += pwrRow("CPU PL1 (long)", fmt.Sprintf("%d W", hovered.PL1Watts))
		s += pwrRow("CPU PL2 (short)", fmt.Sprintf("%d W", hovered.PL2Watts))
		s += pwrRow("GPU max clock", gpuMax)
		s += pwrRow("GPU power limit", fmt.Sprintf("%d W", hovered.GPUWatts))
	}

	s += pwrSection.Render("─── Live System ──────────────────") + "\n"
	s += pwrRow("CPU clock", fmt.Sprintf("%d MHz  (hw max %d MHz)", si.CPUCurMHz, si.CPUMaxMHz))
	s += pwrRow("CPU TDP", fmt.Sprintf("PL1 %dW  PL2 %dW", si.PL1Watts, si.PL2Watts))
	s += pwrRow("CPU temp", fmt.Sprintf("%d°C", si.CPUTempPkg))
	s += pwrRow("CPU fan", fmt.Sprintf("%d RPM", si.CPUFanRPM))
	if si.GPUName != "" {
		s += pwrSection.Render("─── GPU ──────────────────────────") + "\n"
		s += pwrRow("GPU", si.GPUName)
		s += pwrRow("GPU clock", fmt.Sprintf("%d MHz  (hw max %d MHz)", si.GPUCurMHz, si.GPUMaxMHz))
		s += pwrRow("GPU power", fmt.Sprintf("%.1f W  (max %.1f W)", si.GPUWatts, si.GPUMaxWatts))
		s += pwrRow("GPU temp", fmt.Sprintf("%d°C", si.GPUTemp))
		s += pwrRow("GPU fan", fmt.Sprintf("%d RPM", si.GPUFanRPM))
	}
	s += pwrDim.Render("\n↑/↓ navigate · enter apply · q quit")
	return s
}

func (m TUIModel) viewCustom() string {
	s := pwrHeader.Render("Custom Profile") + "\n\n"
	for i, f := range m.fields {
		val := f.value
		if i == m.fieldIdx {
			if m.inputMode {
				val = m.inputBuf + pwrCursor.Render("█")
			}
			s += pwrActive.Render("▶ ") + pwrLabel.Render(f.label) + pwrValue.Render(val) + "\n"
		} else {
			s += "  " + pwrLabel.Render(f.label) + pwrDim.Render(val) + "\n"
		}
	}
	if m.errMsg != "" {
		s += "\n" + pwrError.Render("Error: "+m.errMsg)
	}
	s += pwrDim.Render("\n↑/↓ navigate · enter edit · s save & apply · q cancel")
	return s
}

func pwrRow(label, val string) string {
	return pwrLabel.Render("  "+label+":") + pwrValue.Render(val) + "\n"
}

func defaultFields() []customField {
	return []customField{
		{label: "Profile name", key: "name", value: "my-profile"},
		{label: "CPU max MHz", key: "cpu_max_mhz", value: "4500"},
		{label: "PL1 watts (long)", key: "pl1", value: "35"},
		{label: "PL2 watts (short)", key: "pl2", value: "45"},
		{label: "GPU max MHz (0=off)", key: "gpu_max_mhz", value: "0"},
		{label: "GPU watts (0=off)", key: "gpu_watts", value: "60"},
	}
}

func buildProfile(fields []customField) (Profile, error) {
	m := map[string]string{}
	for _, f := range fields {
		m[f.key] = strings.TrimSpace(f.value)
	}
	if m["name"] == "" {
		return Profile{}, fmt.Errorf("name required")
	}
	pi := func(k string) (int, error) { return strconv.Atoi(m[k]) }
	cpu, err := pi("cpu_max_mhz")
	if err != nil {
		return Profile{}, fmt.Errorf("invalid cpu_max_mhz")
	}
	pl1, err := pi("pl1")
	if err != nil {
		return Profile{}, fmt.Errorf("invalid pl1")
	}
	pl2, err := pi("pl2")
	if err != nil {
		return Profile{}, fmt.Errorf("invalid pl2")
	}
	gpu, err := pi("gpu_max_mhz")
	if err != nil {
		return Profile{}, fmt.Errorf("invalid gpu_max_mhz")
	}
	gpuW, err := pi("gpu_watts")
	if err != nil {
		return Profile{}, fmt.Errorf("invalid gpu_watts")
	}
	return Profile{
		Name: m["name"], CPUMaxMHz: cpu,
		PL1Watts: pl1, PL2Watts: pl2,
		GPUMaxMHz: gpu, GPUWatts: gpuW,
	}, nil
}

// IsBuiltin returns true if name matches a built-in profile.
func IsBuiltin(name string) bool {
	for _, p := range BuiltinProfiles {
		if p.Name == name {
			return true
		}
	}
	return false
}
