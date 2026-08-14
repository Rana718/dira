package container

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Rana718/dira/internal/tui"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Styles ──────────────────────────────────────────────────────────────────

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
	ctBlue   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	ctOrange = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	ctPaused = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	ctTabAct = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Underline(true)
	ctTabInc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ctInput  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// ─── Tab enum ────────────────────────────────────────────────────────────────

type Tab int

const (
	TabContainers Tab = iota
	TabImages
	TabVolumes
	TabNetworks
)

var tabNames = []string{"Containers", "Images", "Volumes", "Networks"}

// ─── Screen / overlay mode ───────────────────────────────────────────────────

type ScreenMode int

const (
	ScreenList ScreenMode = iota
	ScreenLogs
	ScreenInfo
	ScreenVolumes  // container's own volume view
	ScreenNetworks // container's own network view
	ScreenHistory
	ScreenRename
	ScreenConfirmDel
	ScreenNetCreate
	ScreenVolCreate
	ScreenNetEdit       // connect container → network
	ScreenNetDisconnect // disconnect container from network
)

// ─── Messages ────────────────────────────────────────────────────────────────

type ContentMsg struct{ Text, Err string }
type StatsTickMsg struct{}
type AllStatsMsg struct{ Stats map[string]Stats }
type ActionDoneMsg struct{ Err error }

func StatsTickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg {
		return StatsTickMsg{}
	})
}

// ─── Model ───────────────────────────────────────────────────────────────────

type Model struct {
	// tab
	ActiveTab Tab

	// containers
	Containers      []Container
	ContainerCursor int
	ContainerFilter string
	ContainerSearch bool
	StatsMap        map[string]Stats

	// images
	Images      []Image
	ImageCursor int
	ImageFilter string
	ImageSearch bool

	// volumes
	Volumes      []Volume
	VolumeCursor int
	VolumeFilter string
	VolumeSearch bool

	// networks
	Networks      []Network
	NetworkCursor int
	NetworkFilter string
	NetworkSearch bool

	// sub-screen / overlay
	Screen  ScreenMode
	VP      viewport.Model
	Loading bool
	Err     string

	// rename / create input
	Input    textinput.Model
	InputCtx string // context label

	// confirm-delete
	ConfirmTarget string // "name (id)"

	// window
	WinW, WinH int
}

func NewModel() Model         { return newModelWithTab(TabContainers) }
func NewImagesModel() Model   { return newModelWithTab(TabImages) }
func NewVolumesModel() Model  { return newModelWithTab(TabVolumes) }
func NewNetworksModel() Model { return newModelWithTab(TabNetworks) }

func newModelWithTab(tab Tab) Model {
	inp := textinput.New()
	inp.CharLimit = 80
	inp.Width = 40

	m := Model{
		ActiveTab:  tab,
		Containers: List(),
		Images:     ListImages(),
		Volumes:    ListVolumes(),
		Networks:   ListNetworks(),
		StatsMap:   map[string]Stats{},
		VP:         viewport.New(80, 20),
		Input:      inp,
	}
	return m
}

func (m Model) Init() tea.Cmd { return StatsTickCmd() }

// ─── helpers ─────────────────────────────────────────────────────────────────

func (m Model) selectedContainer() *Container {
	filtered := m.filteredContainers()
	if len(filtered) == 0 || m.ContainerCursor >= len(filtered) {
		return nil
	}
	c := filtered[m.ContainerCursor]
	return &c
}

func (m Model) selectedImage() *Image {
	filtered := m.filteredImages()
	if len(filtered) == 0 || m.ImageCursor >= len(filtered) {
		return nil
	}
	img := filtered[m.ImageCursor]
	return &img
}

func (m Model) selectedVolume() *Volume {
	filtered := m.filteredVolumes()
	if len(filtered) == 0 || m.VolumeCursor >= len(filtered) {
		return nil
	}
	v := filtered[m.VolumeCursor]
	return &v
}

func (m Model) selectedNetwork() *Network {
	filtered := m.filteredNetworks()
	if len(filtered) == 0 || m.NetworkCursor >= len(filtered) {
		return nil
	}
	n := filtered[m.NetworkCursor]
	return &n
}

func (m Model) filteredContainers() []Container {
	if m.ContainerFilter == "" {
		return m.Containers
	}
	f := strings.ToLower(m.ContainerFilter)
	var out []Container
	for _, c := range m.Containers {
		if strings.Contains(strings.ToLower(c.Name), f) ||
			strings.Contains(strings.ToLower(c.Image), f) ||
			strings.Contains(strings.ToLower(c.ID), f) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) filteredImages() []Image {
	if m.ImageFilter == "" {
		return m.Images
	}
	f := strings.ToLower(m.ImageFilter)
	var out []Image
	for _, img := range m.Images {
		if strings.Contains(strings.ToLower(img.Repo), f) ||
			strings.Contains(strings.ToLower(img.Tag), f) ||
			strings.Contains(strings.ToLower(img.ID), f) {
			out = append(out, img)
		}
	}
	return out
}

func (m Model) filteredVolumes() []Volume {
	if m.VolumeFilter == "" {
		return m.Volumes
	}
	f := strings.ToLower(m.VolumeFilter)
	var out []Volume
	for _, v := range m.Volumes {
		if strings.Contains(strings.ToLower(v.Name), f) ||
			strings.Contains(strings.ToLower(v.Driver), f) {
			out = append(out, v)
		}
	}
	return out
}

func (m Model) filteredNetworks() []Network {
	if m.NetworkFilter == "" {
		return m.Networks
	}
	f := strings.ToLower(m.NetworkFilter)
	var out []Network
	for _, n := range m.Networks {
		if strings.Contains(strings.ToLower(n.Name), f) ||
			strings.Contains(strings.ToLower(n.Driver), f) {
			out = append(out, n)
		}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.WinW, m.WinH = msg.Width, msg.Height
		m.VP.Width = msg.Width - 2
		m.VP.Height = msg.Height - 7
		return m, nil

	case StatsTickMsg:
		cs := m.Containers
		return m, func() tea.Msg {
			result := map[string]Stats{}
			for _, c := range cs {
				if !c.Running {
					continue
				}
				if s, err := GetStats(c.Runtime, c.ID); err == nil {
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
			m.Err = ""
			m.Containers = List()
			m.Images = ListImages()
			m.Volumes = ListVolumes()
			m.Networks = ListNetworks()
		}
		return m, nil

	case tea.KeyMsg:
		// overlay screens first
		if m.Screen == ScreenRename || m.Screen == ScreenNetCreate || m.Screen == ScreenVolCreate {
			return m.updateInput(msg)
		}
		if m.Screen == ScreenConfirmDel {
			return m.updateConfirmDel(msg)
		}
		if m.Screen != ScreenList {
			return m.updateSubView(msg)
		}
		// search modes
		switch m.ActiveTab {
		case TabContainers:
			if m.ContainerSearch {
				return m.updateContainerSearch(msg)
			}
		case TabImages:
			if m.ImageSearch {
				return m.updateImageSearch(msg)
			}
		case TabVolumes:
			if m.VolumeSearch {
				return m.updateVolumeSearch(msg)
			}
		case TabNetworks:
			if m.NetworkSearch {
				return m.updateNetworkSearch(msg)
			}
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m Model) updateSubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.Screen = ScreenList
		m.Err = ""
		m.Loading = false
		m.VP.SetContent("")
		return m, nil
	}
	var cmd tea.Cmd
	m.VP, cmd = m.VP.Update(msg)
	return m, cmd
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.Screen = ScreenList
		m.Input.SetValue("")
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.Input.Value())
		m.Input.SetValue("")
		m.Screen = ScreenList
		if val == "" {
			return m, nil
		}
		switch m.InputCtx {
		case "rename":
			c := m.selectedContainer()
			if c == nil {
				return m, nil
			}
			rt, id := c.Runtime, c.ID
			return m, func() tea.Msg { return ActionDoneMsg{Err: RenameContainer(rt, id, val)} }
		case "net-create":
			rt := m.activeRuntime()
			return m, func() tea.Msg { return ActionDoneMsg{Err: CreateNetwork(rt, val, "bridge")} }
		case "vol-create":
			rt := m.activeRuntime()
			return m, func() tea.Msg { return ActionDoneMsg{Err: CreateVolume(rt, val)} }
		case "net-connect":
			net := m.selectedNetwork()
			if net == nil {
				return m, nil
			}
			rt, netName := net.Runtime, net.Name
			return m, func() tea.Msg { return ActionDoneMsg{Err: ConnectNetwork(rt, netName, val)} }
		case "net-connect-to":
			net := m.selectedNetwork()
			if net == nil {
				return m, nil
			}
			rt, netName := net.Runtime, net.Name
			return m, func() tea.Msg { return ActionDoneMsg{Err: ConnectNetwork(rt, netName, val)} }
		case "net-disconnect":
			net := m.selectedNetwork()
			if net == nil {
				return m, nil
			}
			rt, netName := net.Runtime, net.Name
			return m, func() tea.Msg { return ActionDoneMsg{Err: DisconnectNetwork(rt, netName, val)} }
		case "net-disconnect-from":
			net := m.selectedNetwork()
			if net == nil {
				return m, nil
			}
			rt, netName := net.Runtime, net.Name
			return m, func() tea.Msg { return ActionDoneMsg{Err: DisconnectNetwork(rt, netName, val)} }
		}
	default:
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateConfirmDel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.Screen = ScreenList
		target := m.ConfirmTarget
		switch m.ActiveTab {
		case TabContainers:
			c := m.selectedContainer()
			if c == nil {
				return m, nil
			}
			rt, id := c.Runtime, c.ID
			return m, func() tea.Msg {
				_ = Stop(rt, id)
				return ActionDoneMsg{Err: Delete(rt, id)}
			}
		case TabImages:
			img := m.selectedImage()
			if img == nil {
				return m, nil
			}
			rt, id := img.Runtime, img.ID
			return m, func() tea.Msg { return ActionDoneMsg{Err: RemoveImage(rt, id, false)} }
		case TabVolumes:
			vol := m.selectedVolume()
			if vol == nil {
				return m, nil
			}
			rt, name := vol.Runtime, vol.Name
			return m, func() tea.Msg { return ActionDoneMsg{Err: RemoveVolume(rt, name)} }
		case TabNetworks:
			net := m.selectedNetwork()
			if net == nil {
				return m, nil
			}
			rt, name := net.Runtime, net.Name
			return m, func() tea.Msg { return ActionDoneMsg{Err: RemoveNetwork(rt, name)} }
		}
		_ = target
	case "n", "N", "esc", "q":
		m.Screen = ScreenList
		m.ConfirmTarget = ""
	}
	return m, nil
}

func (m Model) activeRuntime() string {
	for _, rt := range []string{"docker", "podman"} {
		if runtimeAvailable(rt) {
			return rt
		}
	}
	return "docker"
}

// ─── Search updates ───────────────────────────────────────────────────────────

func (m Model) updateContainerSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.ContainerSearch = false
	case "backspace":
		if len(m.ContainerFilter) > 0 {
			m.ContainerFilter = m.ContainerFilter[:len(m.ContainerFilter)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.ContainerFilter += msg.String()
		}
	}
	m.ContainerCursor = 0
	return m, nil
}

func (m Model) updateImageSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.ImageSearch = false
	case "backspace":
		if len(m.ImageFilter) > 0 {
			m.ImageFilter = m.ImageFilter[:len(m.ImageFilter)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.ImageFilter += msg.String()
		}
	}
	m.ImageCursor = 0
	return m, nil
}

func (m Model) updateVolumeSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.VolumeSearch = false
	case "backspace":
		if len(m.VolumeFilter) > 0 {
			m.VolumeFilter = m.VolumeFilter[:len(m.VolumeFilter)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.VolumeFilter += msg.String()
		}
	}
	m.VolumeCursor = 0
	return m, nil
}

func (m Model) updateNetworkSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.NetworkSearch = false
	case "backspace":
		if len(m.NetworkFilter) > 0 {
			m.NetworkFilter = m.NetworkFilter[:len(m.NetworkFilter)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.NetworkFilter += msg.String()
		}
	}
	m.NetworkCursor = 0
	return m, nil
}

// ─── Main list update ─────────────────────────────────────────────────────────

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.Err = ""
	key := msg.String()

	// global tab switching
	switch key {
	case "1":
		m.ActiveTab = TabContainers
		return m, nil
	case "2":
		m.ActiveTab = TabImages
		return m, nil
	case "3":
		m.ActiveTab = TabVolumes
		return m, nil
	case "4":
		m.ActiveTab = TabNetworks
		return m, nil
	case "tab":
		m.ActiveTab = (m.ActiveTab + 1) % 4
		return m, nil
	case "shift+tab":
		m.ActiveTab = (m.ActiveTab + 3) % 4
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}

	switch m.ActiveTab {
	case TabContainers:
		return m.updateContainerList(key)
	case TabImages:
		return m.updateImageList(key)
	case TabVolumes:
		return m.updateVolumeList(key)
	case TabNetworks:
		return m.updateNetworkList(key)
	}
	return m, nil
}

// ─── Container list ───────────────────────────────────────────────────────────

func (m Model) updateContainerList(key string) (tea.Model, tea.Cmd) {
	filtered := m.filteredContainers()
	c := m.selectedContainer()

	switch key {
	case "up", "k":
		m.ContainerCursor = clamp(m.ContainerCursor-1, 0, len(filtered)-1)
	case "down", "j":
		m.ContainerCursor = clamp(m.ContainerCursor+1, 0, len(filtered)-1)
	case "/":
		m.ContainerSearch = true
		m.ContainerFilter = ""
	case "esc":
		m.ContainerFilter = ""
	case "r":
		m.Containers = List()
	// actions
	case "S": // start
		if c == nil || c.Running {
			break
		}
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: Start(rt, id)} }
	case "s": // stop
		if c == nil || !c.Running {
			break
		}
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: Stop(rt, id)} }
	case "R": // restart
		if c == nil {
			break
		}
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: RestartContainer(rt, id)} }
	case "p": // pause / unpause
		if c == nil {
			break
		}
		m.Loading = true
		rt, id, running := c.Runtime, c.ID, c.Running
		isPaused := strings.Contains(strings.ToLower(c.Status), "pause")
		return m, func() tea.Msg {
			var err error
			if isPaused {
				err = UnpauseContainer(rt, id)
			} else if running {
				err = PauseContainer(rt, id)
			}
			return ActionDoneMsg{Err: err}
		}
	case "d": // force delete
		if c == nil {
			break
		}
		m.Screen = ScreenConfirmDel
		m.ConfirmTarget = fmt.Sprintf("%s (%s)", c.Name, c.ID)
	case "e": // exec shell
		if c == nil || !c.Running {
			m.Err = "container is not running"
			break
		}
		return m, tea.ExecProcess(
			exec.Command(c.Runtime, "exec", "-it", c.ID, "/bin/sh"),
			func(err error) tea.Msg { return ActionDoneMsg{Err: err} },
		)
	case "l": // logs
		if c == nil {
			break
		}
		m.Screen = ScreenLogs
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			raw, err := Logs(rt, id, 300)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			return ContentMsg{Text: tui.ColorizeLogs(raw), Err: errStr}
		}
	case "i": // inspect
		if c == nil {
			break
		}
		m.Screen = ScreenInfo
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			info, err := Inspect(rt, id)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: RenderInfo(info)}
		}
	case "v": // container volumes
		if c == nil {
			break
		}
		m.Screen = ScreenVolumes
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			info, err := Inspect(rt, id)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: RenderVolumes(info)}
		}
	case "n": // container networks
		if c == nil {
			break
		}
		m.Screen = ScreenNetworks
		m.Loading = true
		rt, id := c.Runtime, c.ID
		return m, func() tea.Msg {
			nets := ContainerNetworks(rt, id)
			return ContentMsg{Text: renderContainerNetworks(nets, id)}
		}
	case "h": // image history
		if c == nil {
			break
		}
		m.Screen = ScreenHistory
		m.Loading = true
		rt, img := c.Runtime, c.Image
		return m, func() tea.Msg {
			layers, err := GetImageHistory(rt, img)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: renderHistory(layers)}
		}
	case "c": // edit file in container
		if c == nil || !c.Running {
			m.Err = "container must be running to edit files"
			break
		}
		rt, id, name := c.Runtime, c.ID, c.Name
		return m, tea.ExecProcess(
			exec.Command("/bin/sh", "-c", editFileScript(rt, id, name)),
			func(err error) tea.Msg { return ActionDoneMsg{Err: err} },
		)
	case "N": // rename
		if c == nil {
			break
		}
		m.Screen = ScreenRename
		m.InputCtx = "rename"
		m.Input.Placeholder = "new container name"
		m.Input.SetValue("")
		m.Input.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

// ─── Image list ───────────────────────────────────────────────────────────────

func (m Model) updateImageList(key string) (tea.Model, tea.Cmd) {
	filtered := m.filteredImages()
	img := m.selectedImage()

	switch key {
	case "up", "k":
		m.ImageCursor = clamp(m.ImageCursor-1, 0, len(filtered)-1)
	case "down", "j":
		m.ImageCursor = clamp(m.ImageCursor+1, 0, len(filtered)-1)
	case "/":
		m.ImageSearch = true
		m.ImageFilter = ""
	case "esc":
		m.ImageFilter = ""
	case "r":
		m.Images = ListImages()
	case "d": // remove image
		if img == nil {
			break
		}
		m.Screen = ScreenConfirmDel
		m.ConfirmTarget = fmt.Sprintf("%s:%s (%s)", img.Repo, img.Tag, img.ID)
	case "D": // force remove image
		if img == nil {
			break
		}
		m.Loading = true
		rt, id := img.Runtime, img.ID
		return m, func() tea.Msg { return ActionDoneMsg{Err: RemoveImage(rt, id, true)} }
	case "P": // prune dangling
		rt := m.activeRuntime()
		m.Loading = true
		return m, func() tea.Msg {
			out, err := PruneImages(rt)
			if err != nil {
				return ActionDoneMsg{Err: err}
			}
			return ContentMsg{Text: ctGreen.Render("  ✓ Pruned:\n") + "  " + out}
		}
	case "i": // inspect
		if img == nil {
			break
		}
		m.Screen = ScreenInfo
		m.Loading = true
		rt, id := img.Runtime, img.ID
		return m, func() tea.Msg {
			text := ImageInspectText(rt, id)
			return ContentMsg{Text: text}
		}
	case "h": // history
		if img == nil {
			break
		}
		m.Screen = ScreenHistory
		m.Loading = true
		rt := img.Runtime
		ref := img.Repo + ":" + img.Tag
		return m, func() tea.Msg {
			layers, err := GetImageHistory(rt, ref)
			if err != nil {
				return ContentMsg{Err: err.Error()}
			}
			return ContentMsg{Text: renderHistory(layers)}
		}
	}
	return m, nil
}

// ─── Volume list ──────────────────────────────────────────────────────────────

func (m Model) updateVolumeList(key string) (tea.Model, tea.Cmd) {
	filtered := m.filteredVolumes()
	vol := m.selectedVolume()

	switch key {
	case "up", "k":
		m.VolumeCursor = clamp(m.VolumeCursor-1, 0, len(filtered)-1)
	case "down", "j":
		m.VolumeCursor = clamp(m.VolumeCursor+1, 0, len(filtered)-1)
	case "/":
		m.VolumeSearch = true
		m.VolumeFilter = ""
	case "esc":
		m.VolumeFilter = ""
	case "r":
		m.Volumes = ListVolumes()
	case "c": // create
		m.Screen = ScreenVolCreate
		m.InputCtx = "vol-create"
		m.Input.Placeholder = "new volume name"
		m.Input.SetValue("")
		m.Input.Focus()
		return m, textinput.Blink
	case "d": // remove
		if vol == nil {
			break
		}
		m.Screen = ScreenConfirmDel
		m.ConfirmTarget = vol.Name
	case "P": // prune
		rt := m.activeRuntime()
		m.Loading = true
		return m, func() tea.Msg {
			out, err := PruneVolumes(rt)
			if err != nil {
				return ActionDoneMsg{Err: err}
			}
			return ContentMsg{Text: ctGreen.Render("  ✓ Pruned:\n") + "  " + out}
		}
	case "i": // inspect (show mountpoint)
		if vol == nil {
			break
		}
		m.Screen = ScreenInfo
		m.Loading = false
		m.VP.SetContent(renderVolumeDetail(vol))
		m.VP.GotoTop()
		m.Screen = ScreenInfo
	}
	return m, nil
}

// ─── Network list ─────────────────────────────────────────────────────────────

func (m Model) updateNetworkList(key string) (tea.Model, tea.Cmd) {
	filtered := m.filteredNetworks()
	net := m.selectedNetwork()

	switch key {
	case "up", "k":
		m.NetworkCursor = clamp(m.NetworkCursor-1, 0, len(filtered)-1)
	case "down", "j":
		m.NetworkCursor = clamp(m.NetworkCursor+1, 0, len(filtered)-1)
	case "/":
		m.NetworkSearch = true
		m.NetworkFilter = ""
	case "esc":
		m.NetworkFilter = ""
	case "r":
		m.Networks = ListNetworks()
	case "c": // create
		m.Screen = ScreenNetCreate
		m.InputCtx = "net-create"
		m.Input.Placeholder = "new network name"
		m.Input.SetValue("")
		m.Input.Focus()
		return m, textinput.Blink
	case "e": // connect container → this network
		if net == nil {
			break
		}
		m.Screen = ScreenNetEdit
		m.InputCtx = "net-connect-to"
		m.Input.Placeholder = "container name or ID"
		m.Input.SetValue("")
		m.Input.Focus()
		return m, textinput.Blink
	case "x": // disconnect container from this network
		if net == nil {
			break
		}
		m.Screen = ScreenNetDisconnect
		m.InputCtx = "net-disconnect-from"
		m.Input.Placeholder = "container name or ID"
		m.Input.SetValue("")
		m.Input.Focus()
		return m, textinput.Blink
	case "d": // remove
		if net == nil {
			break
		}
		m.Screen = ScreenConfirmDel
		m.ConfirmTarget = net.Name
	case "i": // inspect
		if net == nil {
			break
		}
		m.Screen = ScreenInfo
		m.Loading = true
		rt, name := net.Runtime, net.Name
		return m, func() tea.Msg {
			text := NetworkInspectText(rt, name)
			return ContentMsg{Text: text}
		}
	}
	return m, nil
}

// ─── View (delegated to tui_view.go) ─────────────────────────────────────────

func (m Model) View() string { return m.Render() }
