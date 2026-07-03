package container

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Run TUI Styles

var (
	runTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	runSel    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	runNorm   = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	runDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	runGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	runRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	runYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	runBlue   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	runLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(14)
	runCat    = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
)

// ── Steps

type RunStep int

const (
	StepRuntime RunStep = iota
	StepPreset
	StepEdit
	StepCustomImage
	StepCustomTag
	StepCustomPorts
	StepConfirm
	StepRunning
	StepDone
)

// ── Edit fields

type EditField int

const (
	EditName EditField = iota
	EditImage
	EditPorts
	EditEnv
	EditFieldCount
)

// ── Messages

type SearchResultMsg struct{ Results []string }
type TagResultMsg struct{ Tags []string }
type RunDoneMsg struct {
	Output string
	Err    error
}

// ── Model

type RunModel struct {
	Step    RunStep
	Cursor  int
	Runtime string
	Preset  Preset

	Runtimes []string

	AllPresets      []Preset
	FilteredPresets []Preset
	FilterInput     textinput.Model
	Filtering       bool

	// edit fields
	EditCursor  EditField
	NameInput   textinput.Model
	EImageInput textinput.Model
	EPortInput  textinput.Model
	EEnvInput   textinput.Model

	// custom image flow
	ImageInput textinput.Model
	TagInput   textinput.Model
	PortInput  textinput.Model

	SearchResults []string
	SearchCursor  int
	Searching     bool

	Tags      []string
	TagCursor int

	ScrollOffset int

	Output string
	Err    string

	WinW, WinH int
}

func NewRunModel() RunModel {
	var runtimes []string
	for _, rt := range []string{"docker", "podman"} {
		if runtimeAvailable(rt) {
			runtimes = append(runtimes, rt)
		}
	}

	allPresets := LoadPresets()

	imgInput := textinput.New()
	imgInput.Placeholder = "e.g. redis, postgres, nginx, myrepo/myimage"
	imgInput.CharLimit = 120
	imgInput.Width = 50

	tagInput := textinput.New()
	tagInput.Placeholder = "e.g. latest, alpine, 7.2, 18-alpine"
	tagInput.CharLimit = 60
	tagInput.Width = 40

	portInput := textinput.New()
	portInput.Placeholder = "e.g. 6379:6379, 8080:80 (comma separated)"
	portInput.CharLimit = 120
	portInput.Width = 50

	filterInput := textinput.New()
	filterInput.Placeholder = "type to filter..."
	filterInput.CharLimit = 60
	filterInput.Width = 40

	nameInput := textinput.New()
	nameInput.Placeholder = "container name (optional)"
	nameInput.CharLimit = 60
	nameInput.Width = 50

	eImgInput := textinput.New()
	eImgInput.CharLimit = 120
	eImgInput.Width = 50

	ePortInput := textinput.New()
	ePortInput.Placeholder = "host:container, host:container"
	ePortInput.CharLimit = 120
	ePortInput.Width = 50

	eEnvInput := textinput.New()
	eEnvInput.Placeholder = "KEY=VAL, KEY2=VAL2"
	eEnvInput.CharLimit = 200
	eEnvInput.Width = 60

	m := RunModel{
		Runtimes:        runtimes,
		AllPresets:      allPresets,
		FilteredPresets: allPresets,
		ImageInput:      imgInput,
		TagInput:        tagInput,
		PortInput:       portInput,
		FilterInput:     filterInput,
		NameInput:       nameInput,
		EImageInput:     eImgInput,
		EPortInput:      ePortInput,
		EEnvInput:       eEnvInput,
	}

	if len(runtimes) == 1 {
		m.Runtime = runtimes[0]
		m.Step = StepPreset
	} else if len(runtimes) == 0 {
		m.Err = "neither docker nor podman found"
		m.Step = StepDone
	}

	return m
}

func (m RunModel) Init() tea.Cmd { return nil }

func (m RunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.WinW = msg.Width
		m.WinH = msg.Height
		return m, nil
	case SearchResultMsg:
		m.Searching = false
		m.SearchResults = msg.Results
		m.SearchCursor = 0
		return m, nil
	case TagResultMsg:
		m.Tags = msg.Tags
		m.TagCursor = 0
		return m, nil
	case RunDoneMsg:
		m.Step = StepDone
		m.Output = msg.Output
		if msg.Err != nil {
			m.Err = msg.Err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// forward to active text inputs
	switch m.Step {
	case StepPreset:
		if m.Filtering {
			var cmd tea.Cmd
			m.FilterInput, cmd = m.FilterInput.Update(msg)
			// only re-filter if value actually changed
			newVal := m.FilterInput.Value()
			newFiltered := FilterPresets(m.AllPresets, newVal)
			if len(newFiltered) != len(m.FilteredPresets) {
				m.FilteredPresets = newFiltered
				m.Cursor = 0
				m.ScrollOffset = 0
			}
			return m, cmd
		}
	case StepEdit:
		return m.updateEditInput(msg)
	case StepCustomImage:
		var cmd tea.Cmd
		m.ImageInput, cmd = m.ImageInput.Update(msg)
		return m, cmd
	case StepCustomTag:
		var cmd tea.Cmd
		m.TagInput, cmd = m.TagInput.Update(msg)
		return m, cmd
	case StepCustomPorts:
		var cmd tea.Cmd
		m.PortInput, cmd = m.PortInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m RunModel) updateEditInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.EditCursor {
	case EditName:
		m.NameInput, cmd = m.NameInput.Update(msg)
	case EditImage:
		m.EImageInput, cmd = m.EImageInput.Update(msg)
	case EditPorts:
		m.EPortInput, cmd = m.EPortInput.Update(msg)
	case EditEnv:
		m.EEnvInput, cmd = m.EEnvInput.Update(msg)
	}
	return m, cmd
}

func (m RunModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// only ctrl+c force-quits from anywhere
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.Step == StepDone {
		return m, tea.Quit
	}

	switch m.Step {
	case StepRuntime:
		return m.handleRuntime(key)
	case StepPreset:
		return m.handlePreset(key, msg)
	case StepEdit:
		return m.handleEdit(key, msg)
	case StepCustomImage:
		return m.handleCustomImage(key, msg)
	case StepCustomTag:
		return m.handleCustomTag(key, msg)
	case StepCustomPorts:
		return m.handleCustomPorts(key, msg)
	case StepConfirm:
		return m.handleConfirm(key)
	}
	return m, nil
}

func (m RunModel) handleRuntime(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Runtimes)-1 {
			m.Cursor++
		}
	case "enter":
		m.Runtime = m.Runtimes[m.Cursor]
		m.Step = StepPreset
		m.Cursor = 0
	case "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m RunModel) handlePreset(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.Filtering {
		switch key {
		case "esc":
			m.Filtering = false
			m.FilterInput.Blur()
			m.FilterInput.SetValue("")
			m.FilteredPresets = m.AllPresets
			m.Cursor = 0
			m.ScrollOffset = 0
			return m, nil
		case "enter":
			if len(m.FilteredPresets) == 0 {
				return m, nil
			}
			m.Filtering = false
			m.FilterInput.Blur()
			p := m.FilteredPresets[m.Cursor]
			if p.Image == "" {
				m.Step = StepCustomImage
				m.ImageInput.Focus()
				return m, textinput.Blink
			}
			m.Preset = p
			m.Step = StepConfirm
			return m, nil
		case "up", "ctrl+p":
			if m.Cursor > 0 {
				m.Cursor--
				m.adjustScroll()
			}
			return m, nil
		case "down", "ctrl+n":
			if m.Cursor < len(m.FilteredPresets)-1 {
				m.Cursor++
				m.adjustScroll()
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.FilterInput, cmd = m.FilterInput.Update(msg)
			newFiltered := FilterPresets(m.AllPresets, m.FilterInput.Value())
			if len(newFiltered) != len(m.FilteredPresets) || !presetsEqual(newFiltered, m.FilteredPresets) {
				m.FilteredPresets = newFiltered
				m.Cursor = 0
				m.ScrollOffset = 0
			}
			return m, cmd
		}
	}

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			m.adjustScroll()
		}
	case "down", "j":
		if m.Cursor < len(m.FilteredPresets)-1 {
			m.Cursor++
			m.adjustScroll()
		}
	case "f", "/":
		m.Filtering = true
		m.FilterInput.Focus()
		m.FilterInput.SetValue("")
		m.Cursor = 0
		m.ScrollOffset = 0
		return m, textinput.Blink
	case "enter":
		if len(m.FilteredPresets) == 0 {
			break
		}
		p := m.FilteredPresets[m.Cursor]
		if p.Image == "" {
			m.Step = StepCustomImage
			m.ImageInput.Focus()
			return m, textinput.Blink
		}
		m.Preset = p
		m.Step = StepConfirm
		return m, nil
	case "e":
		if len(m.FilteredPresets) == 0 {
			break
		}
		p := m.FilteredPresets[m.Cursor]
		if p.Image == "" {
			break
		}
		m.Preset = p
		m.enterEdit()
		return m, textinput.Blink
	case "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

// enterEdit prepares the edit screen with current preset values.
func (m *RunModel) enterEdit() {
	m.Step = StepEdit
	m.EditCursor = EditName

	// pre-fill edit fields
	m.NameInput.SetValue(sanitizeName(m.Preset.Label))
	m.EImageInput.SetValue(m.Preset.Image)
	if len(m.Preset.Ports) > 0 {
		m.EPortInput.SetValue(strings.Join(m.Preset.Ports, ", "))
	} else {
		m.EPortInput.SetValue("")
	}
	if len(m.Preset.Env) > 0 {
		var envParts []string
		for k, v := range m.Preset.Env {
			envParts = append(envParts, k+"="+v)
		}
		m.EEnvInput.SetValue(strings.Join(envParts, ", "))
	} else {
		m.EEnvInput.SetValue("")
	}

	m.NameInput.Focus()
	m.EImageInput.Blur()
	m.EPortInput.Blur()
	m.EEnvInput.Blur()
}

func (m RunModel) handleEdit(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// go back to preset list
		m.Step = StepPreset
		m.Cursor = 0
		m.ScrollOffset = 0
		m.FilteredPresets = m.AllPresets
		m.FilterInput.SetValue("")
		return m, nil
	case "tab", "down":
		m.blurAllEdit()
		m.EditCursor = (m.EditCursor + 1) % EditFieldCount
		m.focusCurrentEdit()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.blurAllEdit()
		m.EditCursor = (m.EditCursor - 1 + EditFieldCount) % EditFieldCount
		m.focusCurrentEdit()
		return m, textinput.Blink
	case "enter":
		// apply edits and go to confirm
		m.applyEdits()
		m.Step = StepConfirm
		return m, nil
	default:
		var cmd tea.Cmd
		switch m.EditCursor {
		case EditName:
			m.NameInput, cmd = m.NameInput.Update(msg)
		case EditImage:
			m.EImageInput, cmd = m.EImageInput.Update(msg)
		case EditPorts:
			m.EPortInput, cmd = m.EPortInput.Update(msg)
		case EditEnv:
			m.EEnvInput, cmd = m.EEnvInput.Update(msg)
		}
		return m, cmd
	}
}

func (m *RunModel) blurAllEdit() {
	m.NameInput.Blur()
	m.EImageInput.Blur()
	m.EPortInput.Blur()
	m.EEnvInput.Blur()
}

func (m *RunModel) focusCurrentEdit() {
	switch m.EditCursor {
	case EditName:
		m.NameInput.Focus()
	case EditImage:
		m.EImageInput.Focus()
	case EditPorts:
		m.EPortInput.Focus()
	case EditEnv:
		m.EEnvInput.Focus()
	}
}

func (m *RunModel) applyEdits() {
	// name
	name := strings.TrimSpace(m.NameInput.Value())
	if name != "" {
		m.Preset.Label = name
	}

	// image
	img := strings.TrimSpace(m.EImageInput.Value())
	if img != "" {
		m.Preset.Image = img
	}

	// ports
	raw := m.EPortInput.Value()
	m.Preset.Ports = nil
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m.Preset.Ports = append(m.Preset.Ports, p)
		}
	}

	// env
	envRaw := m.EEnvInput.Value()
	m.Preset.Env = nil
	if envRaw != "" {
		m.Preset.Env = map[string]string{}
		for _, pair := range strings.Split(envRaw, ",") {
			pair = strings.TrimSpace(pair)
			if eq := strings.IndexByte(pair, '='); eq > 0 {
				m.Preset.Env[pair[:eq]] = pair[eq+1:]
			}
		}
	}
}

func (m *RunModel) adjustScroll() {
	visible := m.visibleLines()
	if m.Cursor < m.ScrollOffset {
		m.ScrollOffset = m.Cursor
	} else if m.Cursor >= m.ScrollOffset+visible {
		m.ScrollOffset = m.Cursor - visible + 1
	}
}

// presetsEqual checks if two preset slices have the same labels (quick check).
func presetsEqual(a, b []Preset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Label != b[i].Label {
			return false
		}
	}
	return true
}

func (m RunModel) visibleLines() int {
	v := m.WinH - 7
	if m.Filtering {
		v -= 2
	}
	if v < 5 {
		v = 5
	}
	return v
}

func (m RunModel) handleCustomImage(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Step = StepPreset
		m.SearchResults = nil
		m.FilteredPresets = m.AllPresets
		m.FilterInput.SetValue("")
		m.Cursor = 0
		m.ScrollOffset = 0
		return m, nil
	case "tab":
		if len(m.SearchResults) > 0 && m.SearchCursor < len(m.SearchResults) {
			m.ImageInput.SetValue(m.SearchResults[m.SearchCursor])
		}
		return m, nil
	case "ctrl+n":
		if m.SearchCursor < len(m.SearchResults)-1 {
			m.SearchCursor++
		}
		return m, nil
	case "ctrl+p":
		if m.SearchCursor > 0 {
			m.SearchCursor--
		}
		return m, nil
	case "ctrl+s":
		query := m.ImageInput.Value()
		if query == "" {
			return m, nil
		}
		m.Searching = true
		rt := m.Runtime
		return m, func() tea.Msg {
			results, _ := SearchImages(rt, query)
			return SearchResultMsg{Results: results}
		}
	case "enter":
		val := m.ImageInput.Value()
		if val == "" {
			return m, nil
		}
		m.Preset = Preset{Image: val, Label: val}
		m.Step = StepCustomTag
		m.TagInput.Focus()
		rt := m.Runtime
		img := val
		return m, tea.Batch(textinput.Blink, func() tea.Msg {
			tags := ListTags(rt, img)
			return TagResultMsg{Tags: tags}
		})
	default:
		var cmd tea.Cmd
		m.ImageInput, cmd = m.ImageInput.Update(msg)
		return m, cmd
	}
}

func (m RunModel) handleCustomTag(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Step = StepCustomImage
		m.ImageInput.Focus()
		return m, textinput.Blink
	case "tab":
		if len(m.Tags) > 0 && m.TagCursor < len(m.Tags) {
			m.TagInput.SetValue(m.Tags[m.TagCursor])
		}
		return m, nil
	case "ctrl+n":
		if m.TagCursor < len(m.Tags)-1 {
			m.TagCursor++
		}
		return m, nil
	case "ctrl+p":
		if m.TagCursor > 0 {
			m.TagCursor--
		}
		return m, nil
	case "enter":
		tag := m.TagInput.Value()
		if tag == "" {
			tag = "latest"
		}
		if !strings.Contains(m.Preset.Image, ":") {
			m.Preset.Image = m.Preset.Image + ":" + tag
		} else {
			parts := strings.SplitN(m.Preset.Image, ":", 2)
			m.Preset.Image = parts[0] + ":" + tag
		}
		m.Step = StepCustomPorts
		m.PortInput.Focus()
		return m, textinput.Blink
	default:
		var cmd tea.Cmd
		m.TagInput, cmd = m.TagInput.Update(msg)
		return m, cmd
	}
}

func (m RunModel) handleCustomPorts(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Step = StepCustomTag
		m.TagInput.Focus()
		return m, textinput.Blink
	case "enter":
		raw := m.PortInput.Value()
		if raw != "" {
			for _, p := range strings.Split(raw, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					m.Preset.Ports = append(m.Preset.Ports, p)
				}
			}
		}
		m.enterEdit()
		return m, textinput.Blink
	default:
		var cmd tea.Cmd
		m.PortInput, cmd = m.PortInput.Update(msg)
		return m, cmd
	}
}

func (m RunModel) handleConfirm(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "enter":
		m.Step = StepRunning
		rt := m.Runtime
		preset := m.Preset
		// fix podman short-name: prefix with docker.io/library/ if needed
		if rt == "podman" {
			preset.Image = QualifyImage(preset.Image)
		}
		return m, func() tea.Msg {
			time.Sleep(200 * time.Millisecond)
			out, err := RunContainer(rt, preset, true)
			return RunDoneMsg{Output: out, Err: err}
		}
	case "e":
		// go back to edit
		m.enterEdit()
		return m, textinput.Blink
	case "q", "esc":
		// go back to preset menu, NOT quit
		m.Step = StepPreset
		m.Cursor = 0
		m.ScrollOffset = 0
		m.FilteredPresets = m.AllPresets
		m.FilterInput.SetValue("")
		return m, nil
	}
	return m, nil
}

// QualifyImage adds docker.io/library/ prefix for podman if the image is a
// short name without a registry (e.g. "redis:8-alpine" → "docker.io/library/redis:8-alpine").
func QualifyImage(img string) string {
	// already has a registry prefix (contains a dot before the first slash)
	if slashIdx := strings.IndexByte(img, '/'); slashIdx > 0 {
		prefix := img[:slashIdx]
		if strings.Contains(prefix, ".") || strings.Contains(prefix, ":") {
			return img // already qualified like docker.io/... or ghcr.io/...
		}
		// has a slash but no dot — like "bitnami/kafka:latest" → "docker.io/bitnami/kafka:latest"
		return "docker.io/" + img
	}
	// no slash at all — official image like "redis:8-alpine"
	return "docker.io/library/" + img
}

// ── Views

func (m RunModel) View() string {
	switch m.Step {
	case StepRuntime:
		return m.viewRuntime()
	case StepPreset:
		return m.viewPreset()
	case StepEdit:
		return m.viewEdit()
	case StepCustomImage:
		return m.viewCustomImage()
	case StepCustomTag:
		return m.viewCustomTag()
	case StepCustomPorts:
		return m.viewCustomPorts()
	case StepConfirm:
		return m.viewConfirm()
	case StepRunning:
		return m.viewRunning()
	case StepDone:
		return m.viewDone()
	}
	return ""
}

func (m RunModel) viewRuntime() string {
	s := runTitle.Render("  Pick container runtime") + "\n\n"
	for i, rt := range m.Runtimes {
		if i == m.Cursor {
			s += runSel.Render("  ▶ "+rt) + "\n"
		} else {
			s += runNorm.Render("    "+rt) + "\n"
		}
	}
	s += "\n" + runDim.Render("  ↑/↓ move · enter select · q quit")
	return s
}

func (m RunModel) viewPreset() string {
	s := runTitle.Render("  Quick Run") + "  " + runDim.Render("["+m.Runtime+"]") + "\n"

	if m.Filtering {
		s += "  " + m.FilterInput.View() + "\n"
	} else {
		s += runDim.Render("  Pick a service. f filter · e edit · enter launch · q quit") + "\n"
	}
	s += "\n"

	visible := m.visibleLines()
	presets := m.FilteredPresets
	end := m.ScrollOffset + visible
	if end > len(presets) {
		end = len(presets)
	}

	if m.ScrollOffset > 0 {
		s += runDim.Render("  ↑ more above") + "\n"
	}

	lastCat := ""
	for i := m.ScrollOffset; i < end; i++ {
		p := presets[i]

		// only show category headers when NOT filtering
		if !m.Filtering && p.Category != "" && p.Category != lastCat {
			lastCat = p.Category
			catLabel := categoryLabel(p.Category)
			if i != m.ScrollOffset || m.ScrollOffset == 0 {
				s += "\n"
			}
			s += "  " + runCat.Render(catLabel) + "\n"
		}

		cursor := "    "
		style := runNorm
		if i == m.Cursor {
			cursor = "  ▶ "
			style = runSel
		}

		if p.Image == "" {
			s += cursor + style.Render(p.Label) + "\n"
			continue
		}

		label := style.Render(p.Label)
		ports := ""
		if len(p.Ports) > 0 {
			ports = runBlue.Render("  " + strings.Join(p.Ports, ", "))
		}
		s += cursor + label + ports + "\n"
	}

	if end < len(presets) {
		s += runDim.Render("  ↓ more below") + "\n"
	}

	if len(presets) == 0 {
		s += runDim.Render("  no matches") + "\n"
	}

	s += "\n"
	if m.Filtering {
		s += runDim.Render("  ↑/↓ move · enter select · esc clear")
	}
	return s
}

func (m RunModel) viewEdit() string {
	s := runTitle.Render("  Edit before launch") + "  " + runDim.Render("["+m.Runtime+"]") + "\n\n"

	type field struct {
		label string
		field EditField
	}
	fields := []field{
		{"Name", EditName},
		{"Image", EditImage},
		{"Ports", EditPorts},
		{"Env", EditEnv},
	}

	for _, f := range fields {
		active := f.field == m.EditCursor
		arrow := "  "
		if active {
			arrow = "▶ "
		}

		var valueStr string
		switch f.field {
		case EditName:
			if active {
				valueStr = m.NameInput.View()
			} else {
				v := m.NameInput.Value()
				if v == "" {
					valueStr = runDim.Render("(auto)")
				} else {
					valueStr = runNorm.Render(v)
				}
			}
		case EditImage:
			if active {
				valueStr = m.EImageInput.View()
			} else {
				valueStr = runBlue.Render(m.EImageInput.Value())
			}
		case EditPorts:
			if active {
				valueStr = m.EPortInput.View()
			} else {
				v := m.EPortInput.Value()
				if v == "" {
					valueStr = runDim.Render("(none)")
				} else {
					valueStr = runYellow.Render(v)
				}
			}
		case EditEnv:
			if active {
				valueStr = m.EEnvInput.View()
			} else {
				v := m.EEnvInput.Value()
				if v == "" {
					valueStr = runDim.Render("(none)")
				} else {
					valueStr = runNorm.Render(v)
				}
			}
		}

		s += "  " + runYellow.Render(arrow) + runLabel.Render(f.label+":") + valueStr + "\n"
	}

	if len(m.Preset.Cmd) > 0 {
		s += "  " + runDim.Render("  Cmd: "+strings.Join(m.Preset.Cmd, " ")) + "\n\n"
	}

	s += runDim.Render("  tab/↓ next field · shift+tab/↑ prev · enter launch · esc back")
	return s
}

func categoryLabel(cat string) string {
	switch cat {
	case "database":
		return "── Databases"
	case "search":
		return "── Search & Vector"
	case "cache":
		return "── Cache"
	case "queue":
		return "── Queues & Messaging"
	case "web":
		return "── Web & Proxy"
	case "storage":
		return "── Storage & Registry"
	case "tool":
		return "── Dev Tools & UI"
	case "infra":
		return "── Infrastructure"
	default:
		return "── " + cat
	}
}

func (m RunModel) viewCustomImage() string {
	s := runTitle.Render("  Custom Image") + "  " + runDim.Render("["+m.Runtime+"]") + "\n\n"
	s += runLabel.Render("  Image:") + m.ImageInput.View() + "\n\n"

	if m.Searching {
		s += runDim.Render("  searching...") + "\n"
	} else if len(m.SearchResults) > 0 {
		s += runDim.Render("  search results (tab select, ctrl+n/p navigate):") + "\n"
		for i, r := range m.SearchResults {
			if i == m.SearchCursor {
				s += runSel.Render("  ▶ "+r) + "\n"
			} else {
				s += runDim.Render("    "+r) + "\n"
			}
		}
	}

	s += "\n" + runDim.Render("  ctrl+s search · tab select · enter confirm · esc back")
	return s
}

func (m RunModel) viewCustomTag() string {
	s := runTitle.Render("  Choose Tag") + "  " + runDim.Render(m.Preset.Image) + "\n\n"
	s += runLabel.Render("  Tag:") + m.TagInput.View() + "\n\n"

	if len(m.Tags) > 0 {
		s += runDim.Render("  available tags (tab select, ctrl+n/p navigate):") + "\n"
		for i, t := range m.Tags {
			if i == m.TagCursor {
				s += runSel.Render("  ▶ "+t) + "\n"
			} else {
				s += runDim.Render("    "+t) + "\n"
			}
		}
	}

	s += "\n" + runDim.Render("  tab select · enter confirm (empty = latest) · esc back")
	return s
}

func (m RunModel) viewCustomPorts() string {
	s := runTitle.Render("  Port Mapping") + "  " + runDim.Render(m.Preset.Image) + "\n\n"
	s += runLabel.Render("  Ports:") + m.PortInput.View() + "\n\n"
	s += runDim.Render("  format: host:container (comma separated)") + "\n"
	s += runDim.Render("  e.g. 6379:6379 or 8080:80, 8443:443") + "\n"
	s += runDim.Render("  leave empty for no port mapping") + "\n"
	s += "\n" + runDim.Render("  enter confirm · esc back")
	return s
}

func (m RunModel) viewConfirm() string {
	s := runTitle.Render("  Ready to launch") + "\n\n"
	s += runLabel.Render("  Name:") + runNorm.Render(sanitizeName(m.Preset.Label)) + "\n"
	s += runLabel.Render("  Runtime:") + runNorm.Render(m.Runtime) + "\n"
	s += runLabel.Render("  Image:") + runBlue.Render(m.Preset.Image) + "\n"
	if len(m.Preset.Ports) > 0 {
		s += runLabel.Render("  Ports:") + runYellow.Render(strings.Join(m.Preset.Ports, ", ")) + "\n"
	}
	if len(m.Preset.Env) > 0 {
		s += runLabel.Render("  Env:") + "\n"
		for k, v := range m.Preset.Env {
			s += "                " + runDim.Render(k+"=") + runNorm.Render(v) + "\n"
		}
	}
	if len(m.Preset.Cmd) > 0 {
		s += runLabel.Render("  Cmd:") + runDim.Render(strings.Join(m.Preset.Cmd, " ")) + "\n"
	}

	// show the actual command that will run (with podman prefix if needed)
	displayPreset := m.Preset
	if m.Runtime == "podman" {
		displayPreset.Image = QualifyImage(displayPreset.Image)
	}
	s += "\n" + runDim.Render("  $ "+BuildRunCommand(m.Runtime, displayPreset, true)) + "\n"
	s += "\n" + runYellow.Render("  enter/y") + runDim.Render(" launch · ") +
		runYellow.Render("e") + runDim.Render(" edit · ") +
		runYellow.Render("esc") + runDim.Render(" back to menu")
	return s
}

func (m RunModel) viewRunning() string {
	return runTitle.Render("  Launching...") + "\n\n" +
		runDim.Render(fmt.Sprintf("  %s run -d %s", m.Runtime, m.Preset.Image))
}

func (m RunModel) viewDone() string {
	if m.Err != "" {
		s := runRed.Render("  ✗ Failed") + "\n\n"
		s += runRed.Render("  "+m.Err) + "\n"
		if m.Output != "" {
			s += "\n" + runDim.Render("  "+m.Output) + "\n"
		}
		s += "\n" + runDim.Render("  press any key to exit")
		return s
	}

	s := runGreen.Render("  ✓ Container started") + "\n\n"
	s += runLabel.Render("  Image:") + runBlue.Render(m.Preset.Image) + "\n"
	if len(m.Preset.Ports) > 0 {
		s += runLabel.Render("  Ports:") + runYellow.Render(strings.Join(m.Preset.Ports, ", ")) + "\n"
	}
	if m.Output != "" {
		id := m.Output
		if len(id) > 12 {
			id = id[:12]
		}
		s += runLabel.Render("  Container:") + runDim.Render(id) + "\n"
	}
	s += runDim.Render("  press any key to exit")
	return s
}
