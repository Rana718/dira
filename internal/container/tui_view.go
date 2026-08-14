package container

import (
	"fmt"
	"strings"

	"github.com/Rana718/dira/internal/helper"
	"github.com/charmbracelet/lipgloss"
)

// col pads a raw string to width, then applies style.
// This is the only safe way to align lipgloss-rendered columns —
// pad first, style second, so ANSI escape bytes never affect column math.
func col(style lipgloss.Style, s string, w int) string {
	return style.Render(helper.Pad(s, w))
}

func colR(style lipgloss.Style, s string, w int) string {
	return style.Render(helper.PadR(s, w))
}

// trunc truncates s to at most n runes, appending "…" if cut.
func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// sep renders the horizontal separator line of exactly winW chars.
func sep(w int) string {
	if w <= 0 {
		w = 80
	}
	return ctBorder.Render(strings.Repeat("─", w))
}

// ─── Top-level Render ────────────────────────────────────────────────────────

func (m Model) Render() string {
	tabBar := m.renderTabBar()
	helpBar := m.renderHelp()

	var body string
	switch m.Screen {
	case ScreenLogs:
		body = m.subView("Logs", "  ↑/↓/PgUp/PgDn scroll · q back")
	case ScreenInfo:
		body = m.subView("Inspect", "  ↑/↓ scroll · q back")
	case ScreenVolumes:
		body = m.subView("Container Volumes", "  ↑/↓ scroll · q back")
	case ScreenNetworks:
		body = m.subView("Container Networks", "  q back")
	case ScreenHistory:
		body = m.subView("Image History", "  ↑/↓ scroll · q back")
	case ScreenImageRun:
		if m.Loading || m.ImageRun == nil {
			body = ctDim.Render("  Reading image configuration...")
		} else {
			body = m.ImageRun.View()
		}
	case ScreenRename:
		body = m.renderInputOverlay("Rename Container", "New name:")
	case ScreenNetCreate:
		body = m.renderInputOverlay("Create Network", "Network name  (driver: bridge):")
	case ScreenVolCreate:
		body = m.renderInputOverlay("Create Volume", "Volume name:")
	case ScreenNetEdit:
		body = m.renderInputOverlay("Connect container → network", "Network name to connect:")
	case ScreenNetDisconnect:
		body = m.renderInputOverlay("Disconnect container from network", "Network name to disconnect:")
	case ScreenConfirmDel:
		body = m.renderConfirm()
	default:
		switch m.ActiveTab {
		case TabContainers:
			body = m.viewContainerList()
		case TabImages:
			body = m.viewImageList()
		case TabVolumes:
			body = m.viewVolumeList()
		case TabNetworks:
			body = m.viewNetworkList()
		}
	}

	// pad to fill terminal height
	used := strings.Count(tabBar, "\n") + 1 +
		strings.Count(body, "\n") + 1 +
		strings.Count(helpBar, "\n") + 1
	fill := m.WinH - used
	if fill < 0 {
		fill = 0
	}
	return tabBar + "\n" + body + strings.Repeat("\n", fill) + helpBar
}

// ─── Tab bar ─────────────────────────────────────────────────────────────────

func (m Model) renderTabBar() string {
	var tabs []string
	for i, name := range tabNames {
		if Tab(i) == m.ActiveTab {
			tabs = append(tabs, ctTabAct.Render(" "+name+" "))
		} else {
			tabs = append(tabs, ctTabInc.Render(" "+name+" "))
		}
	}
	bar := "  " + strings.Join(tabs, ctBorder.Render(" │ "))
	hint := ctDim.Render("   1-4 · tab/shift+tab")
	return bar + hint + "\n" + sep(m.WinW)
}

// ─── Sub-view (logs / inspect scrollable) ────────────────────────────────────

func (m Model) subView(title, helpHint string) string {
	hdr := ctHdr.Render("── " + title + " " + strings.Repeat("─", max(0, 50-len(title))))
	pct := ctDim.Render(fmt.Sprintf(" %d%%", scrollPct(m.VP)))
	var body string
	switch {
	case m.Loading:
		body = ctDim.Render("  Loading…")
	case m.Err != "":
		body = ctRed.Render("  Error: " + m.Err)
	default:
		body = m.VP.View()
	}
	return hdr + "\n" + body + "\n" + ctHelp.Render(helpHint) + pct
}

// ─── Input overlay ────────────────────────────────────────────────────────────

func (m Model) renderInputOverlay(title, label string) string {
	hdr := ctHdr.Render("── " + title + " " + strings.Repeat("─", max(0, 50-len(title))))
	s := hdr + "\n\n"
	s += "  " + ctDim.Render(label) + "\n"
	s += "  " + ctInput.Render("▸ ") + m.Input.View() + "\n\n"
	s += ctDim.Render("  enter  confirm   ·   esc  cancel")
	return s
}

// ─── Confirm delete ───────────────────────────────────────────────────────────

func (m Model) renderConfirm() string {
	s := ctRed.Render("── Delete ── are you sure? "+strings.Repeat("─", 20)) + "\n\n"
	s += "  " + ctValue.Render(m.ConfirmTarget) + "\n\n"
	s += ctRed.Render("  ⚠  This cannot be undone.") + "\n\n"
	s += "  " + ctYellow.Render("y") + ctDim.Render("  confirm") +
		"   " + ctYellow.Render("n / esc") + ctDim.Render("  cancel")
	return s
}

// ─── Help bar ─────────────────────────────────────────────────────────────────

func (m Model) renderHelp() string {
	var keys [][]string

	switch m.Screen {
	case ScreenList:
		switch m.ActiveTab {
		case TabContainers:
			keys = [][]string{
				{"l", "logs"}, {"i", "inspect"}, {"e", "shell"}, {"c", "files"},
				{"v", "volumes"}, {"n", "networks"}, {"h", "history"},
				{"S", "start"}, {"s", "stop"}, {"R", "restart"}, {"p", "pause"},
				{"N", "rename"}, {"d", "delete"}, {"/", "filter"}, {"r", "refresh"}, {"q", "quit"},
			}
		case TabImages:
			keys = [][]string{
				{"S", "start image"}, {"i", "inspect"}, {"h", "history"},
				{"d", "remove"}, {"D", "force-rm"}, {"P", "prune dangling"},
				{"/", "filter"}, {"r", "refresh"}, {"q", "quit"},
			}
		case TabVolumes:
			keys = [][]string{
				{"c", "create"}, {"i", "inspect"},
				{"d", "remove"}, {"P", "prune unused"},
				{"/", "filter"}, {"r", "refresh"}, {"q", "quit"},
			}
		case TabNetworks:
			keys = [][]string{
				{"c", "create"}, {"i", "inspect"},
				{"e", "connect ctr"}, {"x", "disconnect ctr"},
				{"d", "remove"},
				{"/", "filter"}, {"r", "refresh"}, {"q", "quit"},
			}
		}
	default:
		keys = [][]string{{"q / esc", "back"}, {"↑/↓", "scroll"}}
	}

	var parts []string
	for _, kv := range keys {
		parts = append(parts, ctYellow.Render(kv[0])+" "+ctHelp.Render(kv[1]))
	}
	return sep(m.WinW) + "\n" +
		"  " + strings.Join(parts, ctDim.Render("  ·  "))
}

// ─── Container list ───────────────────────────────────────────────────────────

func (m Model) viewContainerList() string {
	filtered := m.filteredContainers()
	s := m.searchBar(m.ContainerSearch, m.ContainerFilter)

	wName, wID, wRT, wStatus := len("NAME"), len("ID"), len("RT"), len("STATUS")
	wPorts, wCPU, wMem, wNet, wBlk := len("PORTS"), len("CPU%"), len("MEM"), len("NET I/O"), len("BLOCK I/O")
	for _, c := range filtered {
		wName = max(wName, helper.Width(c.Name))
		wID = max(wID, helper.Width(c.ID))
		rt := c.Runtime
		if rt == "" {
			rt = "docker"
		}
		wRT = max(wRT, helper.Width(rt))
		status, _ := containerStatus(c)
		wStatus = max(wStatus, helper.Width(status))
		for _, port := range parsePorts(c.Ports, 0) {
			wPorts = max(wPorts, helper.Width(port))
		}
		if st, ok := m.StatsMap[c.ID]; ok && c.Running {
			wCPU = max(wCPU, helper.Width(st.CPU))
			mem := strings.SplitN(st.MemUsage, " / ", 2)[0]
			wMem = max(wMem, helper.Width(mem))
			wNet = max(wNet, helper.Width(st.NetIO))
			wBlk = max(wBlk, helper.Width(st.BlockIO))
		}
	}

	// header — use same widths
	s += ctHdr.Render(
		"  "+
			helper.Pad("NAME", wName)+"  "+
			helper.Pad("ID", wID)+"  "+
			helper.Pad("RT", wRT)+"  "+
			helper.Pad("STATUS", wStatus)+"  "+
			helper.Pad("PORTS", wPorts)+"  "+
			helper.Pad("CPU%", wCPU)+"  "+
			helper.Pad("MEM", wMem)+"  "+
			helper.Pad("NET I/O", wNet)+"  "+
			helper.Pad("BLOCK I/O", wBlk),
	) + "\n"
	s += sep(m.WinW) + "\n"

	if len(filtered) == 0 {
		return s + ctDim.Render("  No containers found  (docker / podman not running?)") + "\n"
	}

	for i, c := range filtered {
		cur := "  "
		nameStyle := ctDim
		if i == m.ContainerCursor {
			cur = "▶ "
			nameStyle = ctSel
		}

		rt := "docker"
		rtStyle := ctDocker
		if c.Runtime == "podman" {
			rt = "podman"
			rtStyle = ctPodman
		}

		statusTxt, statusStyle := containerStatus(c)

		portsLines := parsePorts(c.Ports, wPorts)

		// stats — default blank placeholders of correct width
		cpuTxt := strings.Repeat(" ", wCPU)
		memTxt := strings.Repeat(" ", wMem)
		netTxt := strings.Repeat(" ", wNet)
		blkTxt := strings.Repeat(" ", wBlk)
		cpuSty, memSty, netSty, blkSty := ctDim, ctDim, ctDim, ctDim

		if st, ok := m.StatsMap[c.ID]; ok && c.Running {
			cpuTxt = st.CPU
			cpuSty = ctCPU
			if p := strings.SplitN(st.MemUsage, " / ", 2); len(p) > 0 {
				memTxt = p[0]
			}
			memSty = ctMem
			if st.NetIO != "" && st.NetIO != "—" {
				netTxt = st.NetIO
				netSty = ctBlue
			}
			if st.BlockIO != "" && st.BlockIO != "—" {
				blkTxt = st.BlockIO
				blkSty = ctOrange
			}
		}

		s += cur +
			col(nameStyle, trunc(c.Name, wName), wName) + "  " +
			col(ctDim, c.ID, wID) + "  " +
			col(rtStyle, rt, wRT) + "  " +
			col(statusStyle, statusTxt, wStatus) + "  " +
			colR(ctNet, portsLines[0], wPorts) + "  " +
			col(cpuSty, trunc(cpuTxt, wCPU), wCPU) + "  " +
			col(memSty, trunc(memTxt, wMem), wMem) + "  " +
			col(netSty, trunc(netTxt, wNet), wNet) + "  " +
			col(blkSty, trunc(blkTxt, wBlk), wBlk) + "\n"

		// extra port lines — indent to PORTS column
		indent := strings.Repeat(" ", 2+wName+2+wID+2+wRT+2+wStatus+2)
		for _, extra := range portsLines[1:] {
			s += indent + colR(ctNet, extra, wPorts) + "\n"
		}
	}

	s += "\n" + m.statusBar(len(filtered), len(m.Containers))
	return s
}

func containerStatus(c Container) (string, lipgloss.Style) {
	lower := strings.ToLower(c.Status)
	switch {
	case strings.Contains(lower, "pause"):
		return "paused", ctPaused
	case c.Running:
		return "running", ctGreen
	case strings.Contains(lower, "exited"):
		return "exited", ctDim
	default:
		return "stopped", ctRed
	}
}

// ─── Image list ───────────────────────────────────────────────────────────────

func (m Model) viewImageList() string {
	filtered := m.filteredImages()
	s := m.searchBar(m.ImageSearch, m.ImageFilter)

	wRepo, wTag, wID, wRT := len("REPOSITORY"), len("TAG"), len("ID"), len("RT")
	wSize, wAge, wUse := len("SIZE"), len("CREATED"), len("USE")
	for _, img := range filtered {
		repo, tag := img.Repo, img.Tag
		rt := img.Runtime
		if rt == "" {
			rt = "docker"
		}
		if repo == "<none>" {
			repo = "<dangling>"
		}
		if tag == "<none>" {
			tag = "—"
		}
		wRepo = max(wRepo, helper.Width(repo))
		wTag = max(wTag, helper.Width(tag))
		wID = max(wID, helper.Width(img.ID))
		wRT = max(wRT, helper.Width(rt))
		wSize = max(wSize, helper.Width(img.Size))
		wAge = max(wAge, helper.Width(img.Created))
	}

	s += ctHdr.Render(
		"  "+
			helper.Pad("REPOSITORY", wRepo)+"  "+
			helper.Pad("TAG", wTag)+"  "+
			helper.Pad("ID", wID)+"  "+
			helper.Pad("RT", wRT)+"  "+
			helper.Pad("SIZE", wSize)+"  "+
			helper.Pad("CREATED", wAge)+"  "+
			helper.Pad("USE", wUse),
	) + "\n"
	s += sep(m.WinW) + "\n"

	if len(filtered) == 0 {
		return s + ctDim.Render("  No images found") + "\n"
	}

	for i, img := range filtered {
		cur := "  "
		repoStyle := ctValue
		if i == m.ImageCursor {
			cur = "▶ "
			repoStyle = ctSel
		}

		rt := "docker"
		rtStyle := ctDocker
		if img.Runtime == "podman" {
			rt = "podman"
			rtStyle = ctPodman
		}

		repo := img.Repo
		repoSty := repoStyle
		if repo == "<none>" {
			repo = "<dangling>"
			repoSty = ctDim
		}

		tag := img.Tag
		tagSty := ctBlue
		if tag == "<none>" {
			tag = "—"
			tagSty = ctDim
		}

		inUseTxt := "—"
		inUseSty := ctDim
		if img.InUse {
			inUseTxt = "●"
			inUseSty = ctGreen
		}

		s += cur +
			col(repoSty, trunc(repo, wRepo), wRepo) + "  " +
			col(tagSty, trunc(tag, wTag), wTag) + "  " +
			col(ctDim, img.ID, wID) + "  " +
			col(rtStyle, rt, wRT) + "  " +
			col(ctValue, trunc(img.Size, wSize), wSize) + "  " +
			col(ctDim, trunc(img.Created, wAge), wAge) + "  " +
			col(inUseSty, inUseTxt, wUse) + "\n"
	}

	s += "\n" + m.statusBar(len(filtered), len(m.Images))
	return s
}

// ─── Volume list ──────────────────────────────────────────────────────────────

func (m Model) viewVolumeList() string {
	filtered := m.filteredVolumes()
	s := m.searchBar(m.VolumeSearch, m.VolumeFilter)

	const maxVolumeName, maxMountpoint = 24, 42
	wName, wDriver, wRT, wMount, wUse := len("NAME"), len("DRIVER"), len("RT"), len("MOUNTPOINT"), len("MOUNTED")
	for _, v := range filtered {
		rt := v.Runtime
		if rt == "" {
			rt = "docker"
		}
		wName = max(wName, helper.Width(trunc(v.Name, maxVolumeName)))
		wDriver = max(wDriver, helper.Width(v.Driver))
		wRT = max(wRT, helper.Width(rt))
		wMount = max(wMount, helper.Width(trunc(v.Mountpoint, maxMountpoint)))
	}

	s += ctHdr.Render(
		"  "+
			helper.Pad("NAME", wName)+"  "+
			helper.Pad("DRIVER", wDriver)+"  "+
			helper.Pad("RT", wRT)+"  "+
			helper.Pad("MOUNTPOINT", wMount)+"  "+
			helper.Pad("MOUNTED", wUse),
	) + "\n"
	s += sep(m.WinW) + "\n"

	if len(filtered) == 0 {
		return s + ctDim.Render("  No volumes found") + "\n"
	}

	for i, v := range filtered {
		cur := "  "
		nameStyle := ctValue
		if i == m.VolumeCursor {
			cur = "▶ "
			nameStyle = ctSel
		}

		rt := "docker"
		rtStyle := ctDocker
		if v.Runtime == "podman" {
			rt = "podman"
			rtStyle = ctPodman
		}

		name := trunc(v.Name, maxVolumeName)
		mp := trunc(v.Mountpoint, maxMountpoint)

		inUseTxt := "no"
		inUseSty := ctDim
		if v.InUse {
			inUseTxt = "yes"
			inUseSty = ctGreen
		}

		s += cur +
			col(nameStyle, name, wName) + "  " +
			col(ctValue, trunc(v.Driver, wDriver), wDriver) + "  " +
			col(rtStyle, rt, wRT) + "  " +
			col(ctDim, mp, wMount) + "  " +
			col(inUseSty, inUseTxt, wUse) + "\n"
	}

	s += "\n" + m.statusBar(len(filtered), len(m.Volumes))
	return s
}

// ─── Network list ─────────────────────────────────────────────────────────────

func (m Model) viewNetworkList() string {
	filtered := m.filteredNetworks()
	s := m.searchBar(m.NetworkSearch, m.NetworkFilter)

	wName, wID, wDriver, wScope, wRT := len("NAME"), len("ID"), len("DRIVER"), len("SCOPE"), len("RT")
	for _, n := range filtered {
		rt := n.Runtime
		if rt == "" {
			rt = "docker"
		}
		wName = max(wName, helper.Width(n.Name))
		wID = max(wID, helper.Width(n.ID))
		wDriver = max(wDriver, helper.Width(n.Driver))
		wScope = max(wScope, helper.Width(n.Scope))
		wRT = max(wRT, helper.Width(rt))
	}

	s += ctHdr.Render(
		"  "+
			helper.Pad("NAME", wName)+"  "+
			helper.Pad("ID", wID)+"  "+
			helper.Pad("DRIVER", wDriver)+"  "+
			helper.Pad("SCOPE", wScope)+"  "+
			helper.Pad("RT", wRT),
	) + "\n"
	s += sep(m.WinW) + "\n"

	if len(filtered) == 0 {
		return s + ctDim.Render("  No networks found") + "\n"
	}

	for i, n := range filtered {
		cur := "  "
		nameStyle := ctValue
		if i == m.NetworkCursor {
			cur = "▶ "
			nameStyle = ctSel
		}

		rt := "docker"
		rtStyle := ctDocker
		if n.Runtime == "podman" {
			rt = "podman"
			rtStyle = ctPodman
		}

		driverSty := ctValue
		switch n.Driver {
		case "bridge":
			driverSty = ctGreen
		case "host", "macvlan", "ipvlan":
			driverSty = ctBlue
		case "overlay":
			driverSty = ctOrange
		case "null", "none":
			driverSty = ctDim
		}

		s += cur +
			col(nameStyle, trunc(n.Name, wName), wName) + "  " +
			col(ctDim, n.ID, wID) + "  " +
			col(driverSty, trunc(n.Driver, wDriver), wDriver) + "  " +
			col(ctDim, trunc(n.Scope, wScope), wScope) + "  " +
			col(rtStyle, rt, wRT) + "\n"
	}

	s += "\n" + m.statusBar(len(filtered), len(m.Networks))
	return s
}

// ─── Shared sub-renders ───────────────────────────────────────────────────────

// searchBar renders the live filter input line.
func (m Model) searchBar(active bool, filter string) string {
	if active {
		return ctBlue.Render("  /") + " " + ctValue.Render(filter) + ctBlue.Render("█") + "\n"
	}
	if filter != "" {
		return ctBlue.Render("  filter: ") + ctValue.Render(filter) +
			ctDim.Render("   esc  clear") + "\n"
	}
	return ""
}

// statusBar shows count + error/loading at the bottom of a list.
func (m Model) statusBar(shown, total int) string {
	s := ctDim.Render(fmt.Sprintf("  %d / %d", shown, total))
	if m.Err != "" {
		s += "   " + ctRed.Render("⚠ "+m.Err)
	} else if m.Loading {
		s += "   " + ctDim.Render("working…")
	}
	return s
}

// ─── RenderInfo / RenderVolumes / renderHistory ───────────────────────────────

// RenderInfo renders a container inspect panel.
func RenderInfo(info InspectInfo) string {
	lw := 20 // label column width
	kv := func(k, v string) string {
		label := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Width(lw).
			Render("  " + k + ":")
		return label + ctValue.Render(v) + "\n"
	}

	s := ctHdr.Render("── Identity ─────────────────────────────────────") + "\n"
	s += kv("ID", info.ID)
	s += kv("Name", info.Name)
	s += kv("Image", info.Image)
	s += kv("Status", info.Status)
	s += kv("Created", info.Created)

	r := info.Resources
	s += "\n" + ctHdr.Render("── Resource Limits ──────────────────────────────") + "\n"
	if r.MemoryBytes > 0 {
		s += kv("Memory", helper.FmtBytes(r.MemoryBytes))
	} else {
		s += kv("Memory", ctDim.Render("unlimited"))
	}
	if r.MemorySwapBytes > 0 && r.MemorySwapBytes != r.MemoryBytes {
		s += kv("Swap", helper.FmtBytes(r.MemorySwapBytes))
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
		s += "\n" + ctHdr.Render("── Mounts ───────────────────────────────────────") + "\n"
		for _, mn := range info.Mounts {
			src := ctDim.Render(trunc(mn.Source, 36))
			dst := ctValue.Render(trunc(mn.Destination, 36))
			mode := ""
			if mn.Mode != "" {
				mode = ctDim.Render("  [" + mn.Mode + "]")
			}
			s += "  " + src + "  →  " + dst + mode + "\n"
		}
	}

	if len(info.Env) > 0 {
		s += "\n" + ctHdr.Render("── Environment ──────────────────────────────────") + "\n"
		for _, e := range info.Env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				s += "  " + ctYellow.Render(parts[0]) + ctDim.Render(" = ") + ctValue.Render(parts[1]) + "\n"
			} else {
				s += "  " + ctDim.Render(e) + "\n"
			}
		}
	}
	return s
}

// RenderVolumes renders the volumes/mounts of a container.
func RenderVolumes(info InspectInfo) string {
	if len(info.Mounts) == 0 {
		return ctDim.Render("  No volumes or bind-mounts attached.")
	}
	lw := 14 // label column width (fixed)
	label := func(k string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(lw).Render("  " + k + ":")
	}
	s := ""
	for i, mn := range info.Mounts {
		s += ctHdr.Render(fmt.Sprintf("  ── Mount %-2d ─────────────────────────────────", i+1)) + "\n"
		s += label("Source") + ctValue.Render(mn.Source) + "\n"
		s += label("Destination") + ctValue.Render(mn.Destination) + "\n"
		if mn.Mode != "" {
			s += label("Mode") + ctValue.Render(mn.Mode) + "\n"
		}
		s += "\n"
	}
	return s
}

func renderHistory(layers []ImageLayer) string {
	if len(layers) == 0 {
		return ctDim.Render("  No history available.")
	}
	s := ""
	for i, l := range layers {
		s += ctHdr.Render(fmt.Sprintf("  %2d.", i+1)) + "  " + ctDim.Render(l.CreatedAt) + "\n"
		s += "      " + ctValue.Render(l.CreatedBy) + "\n"
		if l.Size != "" && l.Size != "0B" && l.Size != "<missing>" {
			s += "      " + ctYellow.Render("size: "+l.Size) + "\n"
		}
		s += "\n"
	}
	return s
}

func renderContainerNetworks(nets []string, id string) string {
	if len(nets) == 0 {
		return ctDim.Render("  No networks attached to "+id+".\n\n") +
			ctDim.Render("  Use  e  in the Networks tab to connect this container.")
	}
	s := ctHdr.Render("── Networks attached to "+id+" ─────────────────") + "\n\n"
	for _, n := range nets {
		s += "  " + ctGreen.Render("●") + "  " + ctValue.Render(n) + "\n"
	}
	s += "\n" + ctDim.Render("  Switch to Networks tab  →  e  connect  ·  x  disconnect")
	return s
}

func renderVolumeDetail(v *Volume) string {
	lw := 16
	kv := func(k, val string) string {
		label := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(lw).Render("  " + k + ":")
		return label + ctValue.Render(val) + "\n"
	}
	s := ctHdr.Render("── Volume: "+v.Name+" ─────────────────────────────") + "\n\n"
	s += kv("Name", v.Name)
	s += kv("Driver", v.Driver)
	s += kv("Mountpoint", v.Mountpoint)
	s += kv("Runtime", v.Runtime)
	if v.InUse {
		s += kv("Status", ctGreen.Render("● in use"))
	} else {
		s += kv("Status", ctDim.Render("○ unused"))
	}
	return s
}
