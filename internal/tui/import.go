package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
)

type importFile struct {
	Name  string
	Path  string
	IsDir bool
}

func (m Model) openImport() (tea.Model, tea.Cmd) {
	fromPicker := m.picking
	m.picking = false
	m.importFromPicker = fromPicker
	m.importing = true
	m.session = nil
	m.planning = false
	m.editing = false
	m.searching = false
	m.preview = nil
	m.importKind = ""
	m.importFocus = "tools"
	if len(m.workspace.Sources) > 0 {
		m.importFocus = "sources"
	}
	m.importSourceCursor = 0
	m.importFileCursor = 0
	m.importToolsCursor = 0
	m.importToolsQuery = ""
	m.importBusy = false
	m.ingestJob = nil
	m.clearDestructiveConfirm("")
	if m.importDir == "" {
		m.importDir = defaultImportDir(m.workspace)
	}
	m.reloadImportFiles()
	m.status = "Library import · loading 5e.tools catalog…"
	return m, m.loadToolsCatalogCmd()
}

func (m *Model) closeImport() {
	m.importing = false
	m.clearDestructiveConfirm("")
	if m.importFromPicker {
		m.importFromPicker = false
		m.openPicker()
		m.status = "Back to library"
		return
	}
	m.status = "Back to campaign"
}

func (m Model) updateImport(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.importBusy {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	if m.deleteConfirm && m.confirmKind == "delete-source" {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "y":
			return m.confirmRemoveImportSource()
		case "n", "esc":
			m.clearDestructiveConfirm("Cancelled")
			return m, nil
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "b":
		if m.importFocus == "tools" && m.importToolsQuery != "" {
			m.importToolsQuery = ""
			m.status = "Filter cleared"
			return m, nil
		}
		m.closeImport()
		return m, nil
	case "tab":
		switch m.importFocus {
		case "sources":
			m.importFocus = "tools"
		case "tools":
			m.importFocus = "files"
		default:
			m.importFocus = "sources"
		}
		return m, nil
	case "ctrl+t":
		m.cycleImportKind()
		return m, nil
	case "t":
		if m.importFocus != "tools" {
			m.cycleImportKind()
			return m, nil
		}
	case "ctrl+r":
		m.status = "Refreshing 5e.tools catalog…"
		return m, m.loadToolsCatalogCmd()
	case "j", "down":
		m.moveImportCursor(1)
		return m, nil
	case "k", "up":
		m.moveImportCursor(-1)
		return m, nil
	case "e":
		if m.importFocus == "sources" {
			return m.toggleImportSource()
		}
	case "d":
		if m.importFocus == "sources" {
			return m.armRemoveImportSource()
		}
	case "enter":
		return m.activateImport()
	case "backspace":
		if m.importFocus == "tools" && m.importToolsQuery != "" {
			m.importToolsQuery = m.importToolsQuery[:len(m.importToolsQuery)-1]
			return m, nil
		}
	}
	if m.importFocus == "tools" && len(msg.String()) == 1 && msg.Text != "" && !strings.ContainsAny(msg.Text, "\n\r\t") {
		m.importToolsQuery += msg.Text
		m.importToolsCursor = 0
		return m, nil
	}
	return m, nil
}

func (m *Model) cycleImportKind() {
	switch m.importKind {
	case "":
		m.importKind = domain.SourceBestiary
	case domain.SourceBestiary:
		m.importKind = domain.SourceAdventure
	case domain.SourceAdventure:
		m.importKind = domain.SourceRules
	default:
		m.importKind = ""
	}
	m.status = "Kind " + importKindLabel(m.importKind)
}

func (m *Model) moveImportCursor(delta int) {
	if m.importFocus == "sources" {
		n := len(m.workspace.Sources)
		if n == 0 {
			return
		}
		m.importSourceCursor = clamp(m.importSourceCursor+delta, 0, n-1)
		return
	}
	if m.importFocus == "tools" {
		n := len(m.filteredTools())
		if n == 0 {
			return
		}
		m.importToolsCursor = clamp(m.importToolsCursor+delta, 0, n-1)
		return
	}
	n := len(m.importFiles)
	if n == 0 {
		return
	}
	m.importFileCursor = clamp(m.importFileCursor+delta, 0, n-1)
}

func (m Model) toggleImportSource() (tea.Model, tea.Cmd) {
	if m.importFocus != "sources" || m.importSourceCursor < 0 || m.importSourceCursor >= len(m.workspace.Sources) {
		return m, nil
	}
	scope := m.workspace.Scope
	id := m.workspace.Sources[m.importSourceCursor].ID
	if m.workspace.SourceEnabled(scope, id) {
		m.workspace.DisableSource(scope.WorldID, scope.CampaignID, id)
		m.status = "Disabled in " + scope.Campaign
	} else {
		m.workspace.EnableSource(scope.WorldID, scope.CampaignID, id)
		m.status = "Enabled in " + scope.Campaign
	}
	m.persistWorkspace()
	m.search = searchsvc.New(m.workspace.Records)
	m.attachReferences()
	m.refreshResults()
	return m, nil
}

func (m Model) armRemoveImportSource() (tea.Model, tea.Cmd) {
	if m.importFocus != "sources" || m.importSourceCursor < 0 || m.importSourceCursor >= len(m.workspace.Sources) {
		m.status = "Select a source to remove"
		return m, nil
	}
	source := m.workspace.Sources[m.importSourceCursor]
	m.deleteConfirm = true
	m.confirmKind = "delete-source"
	m.status = "Remove “" + source.Title + "” and its ingested records?  y confirm · n/Esc cancel"
	return m, nil
}

func (m Model) confirmRemoveImportSource() (tea.Model, tea.Cmd) {
	if m.importSourceCursor < 0 || m.importSourceCursor >= len(m.workspace.Sources) {
		m.clearDestructiveConfirm("Nothing selected")
		return m, nil
	}
	source := m.workspace.Sources[m.importSourceCursor]
	m.clearDestructiveConfirm("")
	if !m.workspace.RemoveSource(source.ID) {
		m.status = "Could not remove " + source.Title
		return m, nil
	}
	if m.importSourceCursor >= len(m.workspace.Sources) {
		m.importSourceCursor = max(0, len(m.workspace.Sources)-1)
	}
	m.search = searchsvc.New(m.workspace.Records)
	m.attachReferences()
	m.persistWorkspace()
	m.ensureBrowserSelection()
	m.refreshResults()
	m.status = "Removed " + source.Title + " · ingest it again from 5e.tools or FILES"
	return m, nil
}

func (m Model) activateImport() (tea.Model, tea.Cmd) {
	if m.importFocus == "sources" {
		return m.toggleImportSource()
	}
	if m.importFocus == "tools" {
		return m.runImportTools()
	}
	if m.importFileCursor < 0 || m.importFileCursor >= len(m.importFiles) {
		return m, nil
	}
	entry := m.importFiles[m.importFileCursor]
	if entry.IsDir {
		m.importDir = entry.Path
		m.importFileCursor = 0
		m.reloadImportFiles()
		return m, nil
	}
	return m.runImportFile(entry.Path)
}

func (m Model) runImportFile(path string) (tea.Model, tea.Cmd) {
	ws := m.workspace
	return m.startIngest(filepath.Base(path), ingest.Options{
		Kind:  m.importKind,
		Scope: ws.Scope,
		Path:  path,
	}, func(opts ingest.Options) (domain.Workspace, ingest.Report, error) {
		return ingest.ApplyFile(ws, path, opts)
	})
}

func (m *Model) reloadImportFiles() {
	dir := m.importDir
	if dir == "" {
		dir = "."
	}
	resolved, err := filepath.Abs(dir)
	if err == nil {
		dir = resolved
	}
	m.importDir = dir
	entries, err := os.ReadDir(dir)
	files := make([]importFile, 0)
	parent := filepath.Dir(dir)
	if parent != dir {
		files = append(files, importFile{Name: "..", Path: parent, IsDir: true})
	}
	if err != nil {
		m.importFiles = files
		m.status = "Cannot read " + dir + ": " + err.Error()
		return
	}
	var dirs, mds []importFile
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		if entry.IsDir() {
			dirs = append(dirs, importFile{Name: name + "/", Path: path, IsDir: true})
			continue
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			mds = append(mds, importFile{Name: name, Path: path, IsDir: false})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	sort.Slice(mds, func(i, j int) bool { return strings.ToLower(mds[i].Name) < strings.ToLower(mds[j].Name) })
	files = append(files, dirs...)
	files = append(files, mds...)
	m.importFiles = files
	if m.importFileCursor >= len(files) {
		m.importFileCursor = max(0, len(files)-1)
	}
}

func defaultImportDir(ws domain.Workspace) string {
	for i := len(ws.Sources) - 1; i >= 0; i-- {
		if path := strings.TrimSpace(ws.Sources[i].Path); path != "" {
			dir := filepath.Dir(path)
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				return dir
			}
		}
	}
	home, _ := os.UserHomeDir()
	candidates := []string{}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, "Downloads"))
	}
	if users, err := os.ReadDir("/mnt/c/Users"); err == nil {
		for _, user := range users {
			if !user.IsDir() || strings.EqualFold(user.Name(), "Public") || strings.EqualFold(user.Name(), "Default") {
				continue
			}
			candidates = append(candidates, filepath.Join("/mnt/c/Users", user.Name(), "Downloads"))
		}
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	if home != "" {
		return home
	}
	wd, _ := os.Getwd()
	return wd
}

func importKindLabel(kind domain.SourceKind) string {
	if kind == "" {
		return "auto"
	}
	return string(kind)
}

func (m Model) renderImport() string {
	width := max(1, m.width)
	height := max(1, m.height)
	kind := "kind " + importKindLabel(m.importKind)
	header := headerStyle.Width(width).Render(
		titleStyle.Render("IMPORT") + "  " +
			mutedStyle.Render("library sources · "+m.workspace.Scope.Label()),
	)
	bodyHeight := max(1, height-2)
	body := m.renderImportBody(width, bodyHeight)
	help := "j/k move  Tab pane  Enter ingest adventure  d remove source  e enable  Ctrl+T kind (" + kind + ")  Esc back  q quit"
	if m.importBusy {
		help = "Ingesting…  q quit"
	}
	if m.status != "" {
		help = m.status + "  ·  " + help
	}
	footer := footerStyle.Width(width).Render(help)
	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return appStyle.Width(width).Height(height).MaxHeight(height).Render(content)
}

func (m Model) renderImportBody(width, height int) string {
	if m.importBusy {
		return m.renderImportProgress(width, height)
	}
	leftWidth := max(22, width/4)
	midWidth := max(28, width/3)
	rightWidth := max(20, width-leftWidth-midWidth)
	left := m.renderImportSources(max(1, leftWidth), height)
	mid := m.renderImportTools(max(1, midWidth), height)
	right := m.renderImportFiles(max(1, rightWidth), height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right)
}

func (m Model) renderImportProgress(width, height int) string {
	snap := ingestSnapshot{title: "import", message: "Starting…"}
	if m.ingestJob != nil {
		snap = m.ingestJob.snapshot()
	}
	inner := max(20, width-6)
	barWidth := min(48, inner)
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("INGESTING"))
	builder.WriteString("  ")
	builder.WriteString(ingestSpinner(snap.elapsed))
	builder.WriteString("\n")
	builder.WriteString(titleStyle.Render(truncateImportLine(snap.title, inner)))
	builder.WriteString("\n\n")
	builder.WriteString(snap.message)
	builder.WriteString("\n")
	meta := formatElapsed(snap.elapsed) + " elapsed"
	if snap.current > 0 && snap.total == 0 {
		meta += fmt.Sprintf(" · %d files", snap.current)
	}
	builder.WriteString(mutedStyle.Render(meta))
	builder.WriteString("\n")
	if snap.total > 0 {
		builder.WriteString(progressBar(snap.current, snap.total, barWidth))
		builder.WriteString("\n")
	}
	if len(snap.log) > 0 {
		builder.WriteString("\n")
		for _, line := range snap.log {
			builder.WriteString(mutedStyle.Render("  " + truncateImportLine(line, inner-2)))
			builder.WriteString("\n")
		}
	}
	return focusedPanelStyle.Width(width).Height(height).MaxHeight(height).Render(builder.String())
}

func (m Model) renderImportSources(width, height int) string {
	style := panelStyle
	if m.importFocus == "sources" {
		style = focusedPanelStyle
	}
	inner := max(8, width-4)
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("SOURCES"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render("enabled for this campaign"))
	builder.WriteString("\n\n")
	if len(m.workspace.Sources) == 0 {
		builder.WriteString(mutedStyle.Render("None yet · ingest a 5e.tools book or .md file"))
	}
	enabled := m.workspace.EnabledSourceIDs(m.workspace.Scope)
	enabledSet := map[string]bool{}
	for _, id := range enabled {
		enabledSet[id] = true
	}
	for index, source := range m.workspace.Sources {
		prefix := "  "
		row := normalItemStyle
		if index == m.importSourceCursor && m.importFocus == "sources" {
			prefix = "▸ "
			row = selectedItemStyle
		}
		mark := "○"
		if enabledSet[source.ID] {
			mark = "●"
		}
		kind := string(source.Kind)
		if kind == "" {
			kind = "source"
		}
		line := fmt.Sprintf("%s%s %s  %s", prefix, mark, source.Title, kind)
		if source.Edition != "" {
			line += " " + source.Edition
		}
		builder.WriteString(row.Render(truncateImportLine(line, inner)))
		builder.WriteString("\n")
	}
	return style.Width(width).Height(height).MaxHeight(height).Render(builder.String())
}

func (m Model) filteredTools() []fivetools.Entry {
	return fivetools.FilterCatalog(m.importTools, m.importToolsQuery)
}

func (m Model) renderImportTools(width, height int) string {
	style := panelStyle
	if m.importFocus == "tools" {
		style = focusedPanelStyle
	}
	inner := max(8, width-4)
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("5E.TOOLS"))
	builder.WriteString("\n")
	filter := "adventures → cached reference"
	if m.importToolsQuery != "" {
		filter = "/" + m.importToolsQuery
	}
	builder.WriteString(mutedStyle.Render(truncateImportLine(filter, inner)))
	builder.WriteString("\n\n")
	entries := m.filteredTools()
	if len(m.importTools) == 0 {
		builder.WriteString(mutedStyle.Render("Catalog not loaded · Ctrl+R retry · markdown still works"))
	} else if len(entries) == 0 {
		builder.WriteString(mutedStyle.Render("No matches"))
	}
	rows := max(3, height-6)
	start := 0
	if m.importToolsCursor >= rows {
		start = m.importToolsCursor - rows + 1
	}
	end := min(len(entries), start+rows)
	for index := start; index < end; index++ {
		entry := entries[index]
		prefix := "  "
		row := normalItemStyle
		if index == m.importToolsCursor && m.importFocus == "tools" {
			prefix = "▸ "
			row = selectedItemStyle
		}
		kind := entry.CatalogKind()
		line := fmt.Sprintf("%s%s  %s %s", prefix, entry.Name, entry.ID, kind)
		builder.WriteString(row.Render(truncateImportLine(line, inner)))
		builder.WriteString("\n")
	}
	return style.Width(width).Height(height).MaxHeight(height).Render(builder.String())
}

func (m Model) renderImportFiles(width, height int) string {
	style := panelStyle
	if m.importFocus == "files" {
		style = focusedPanelStyle
	}
	inner := max(8, width-4)
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("FILES"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render("markdown → owner wiki"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(truncateImportLine(m.importDir, inner)))
	builder.WriteString("\n\n")
	if len(m.importFiles) == 0 {
		builder.WriteString(mutedStyle.Render("No folders or markdown files"))
	}
	rows := max(3, height-6)
	start := 0
	if m.importFileCursor >= rows {
		start = m.importFileCursor - rows + 1
	}
	end := min(len(m.importFiles), start+rows)
	for index := start; index < end; index++ {
		entry := m.importFiles[index]
		prefix := "  "
		row := normalItemStyle
		if index == m.importFileCursor && m.importFocus == "files" {
			prefix = "▸ "
			row = selectedItemStyle
		}
		name := entry.Name
		if entry.IsDir && name != ".." && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		builder.WriteString(row.Render(truncateImportLine(prefix+name, inner)))
		builder.WriteString("\n")
	}
	return style.Width(width).Height(height).MaxHeight(height).Render(builder.String())
}

func truncateImportLine(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}

func ingestStatus(name string) string {
	return "Ingesting " + name + "…"
}

const ingestTickEvery = 200 * time.Millisecond

type ingestTickMsg struct{}

func tickIngestProgress() tea.Cmd {
	return tea.Tick(ingestTickEvery, func(time.Time) tea.Msg {
		return ingestTickMsg{}
	})
}

type ingestJob struct {
	mu      sync.Mutex
	title   string
	stage   ingest.Stage
	message string
	current int
	total   int
	started time.Time
	log     []string
	done    bool
	ws      domain.Workspace
	report  ingest.Report
	err     error
}

type ingestSnapshot struct {
	title   string
	stage   ingest.Stage
	message string
	current int
	total   int
	elapsed time.Duration
	log     []string
	done    bool
	ws      domain.Workspace
	report  ingest.Report
	err     error
}

func newIngestJob(title string) *ingestJob {
	return &ingestJob{title: title, started: time.Now(), message: "Starting…"}
}

func (j *ingestJob) onProgress(p ingest.Progress) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.stage = p.Stage
	j.message = p.Message
	j.current = p.Current
	j.total = p.Total
	line := strings.TrimSpace(p.Message)
	if line == "" {
		return
	}
	if n := len(j.log); n == 0 || j.log[n-1] != line {
		j.log = append(j.log, line)
		if len(j.log) > 8 {
			j.log = j.log[len(j.log)-8:]
		}
	}
}

func (j *ingestJob) finish(ws domain.Workspace, report ingest.Report, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.ws = ws
	j.report = report
	j.err = err
	j.done = true
}

func (j *ingestJob) snapshot() ingestSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return ingestSnapshot{
		title:   j.title,
		stage:   j.stage,
		message: j.message,
		current: j.current,
		total:   j.total,
		elapsed: time.Since(j.started),
		log:     append([]string(nil), j.log...),
		done:    j.done,
		ws:      j.ws,
		report:  j.report,
		err:     j.err,
	}
}

func (s ingestSnapshot) statusLine() string {
	elapsed := formatElapsed(s.elapsed)
	if s.message != "" {
		return s.title + " · " + s.message + " · " + elapsed
	}
	return "Ingesting " + s.title + " · " + elapsed
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)
	return fmt.Sprintf("%d:%02d", int(d/time.Minute), int(d%time.Minute/time.Second))
}

func ingestSpinner(d time.Duration) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := int(d / (120 * time.Millisecond))
	if i < 0 {
		i = 0
	}
	return frames[i%len(frames)]
}

func progressBar(cur, total, width int) string {
	if width < 4 {
		return ""
	}
	if total <= 0 {
		total = 1
	}
	if cur < 0 {
		cur = 0
	}
	if cur > total {
		cur = total
	}
	fill := cur * width / total
	return "[" + strings.Repeat("█", fill) + strings.Repeat("░", width-fill) + "]"
}

type toolsCatalogMsg struct {
	entries []fivetools.Entry
	err     error
}

type toolsIngestMsg struct {
	ws     domain.Workspace
	report ingest.Report
	err    error
}

func (m Model) loadToolsCatalogCmd() tea.Cmd {
	fetcher := fivetools.ResolveFetcher(m.toolsFetcher, "")
	return func() tea.Msg {
		entries, err := fivetools.LoadCatalog(fetcher)
		return toolsCatalogMsg{entries: entries, err: err}
	}
}

func (m Model) handleToolsCatalog(msg toolsCatalogMsg) (tea.Model, tea.Cmd) {
	if !m.importing {
		return m, nil
	}
	if msg.err != nil {
		m.status = "5e.tools catalog failed: " + msg.err.Error()
		return m, nil
	}
	m.importTools = msg.entries
	if m.importToolsCursor >= len(m.filteredTools()) {
		m.importToolsCursor = max(0, len(m.filteredTools())-1)
	}
	m.status = fmt.Sprintf("5e.tools · %d books · Enter ingest · Esc back", len(m.importTools))
	return m, nil
}

func (m Model) runImportTools() (tea.Model, tea.Cmd) {
	entries := m.filteredTools()
	if m.importToolsCursor < 0 || m.importToolsCursor >= len(entries) {
		return m, nil
	}
	entry := entries[m.importToolsCursor]
	ref := entry.PageURL()
	ws := m.workspace
	return m.startIngest(entry.Name+" from 5e.tools", ingest.Options{
		Kind:    m.importKind,
		Scope:   ws.Scope,
		Path:    ref,
		Fetcher: m.toolsFetcher,
	}, func(opts ingest.Options) (domain.Workspace, ingest.Report, error) {
		return ingest.ApplyFiveE(ws, ref, opts)
	})
}

func (m Model) startIngest(name string, opts ingest.Options, run func(ingest.Options) (domain.Workspace, ingest.Report, error)) (tea.Model, tea.Cmd) {
	job := newIngestJob(name)
	opts.Progress = job.onProgress
	m.importBusy = true
	m.ingestJob = job
	m.status = ingestStatus(name)
	go func() {
		job.finish(run(opts))
	}()
	return m, tickIngestProgress()
}

func (m Model) handleIngestTick() (tea.Model, tea.Cmd) {
	if !m.importBusy || m.ingestJob == nil {
		return m, nil
	}
	snap := m.ingestJob.snapshot()
	m.status = snap.statusLine()
	if snap.done {
		return m.handleToolsIngest(toolsIngestMsg{ws: snap.ws, report: snap.report, err: snap.err})
	}
	return m, tickIngestProgress()
}

func (m Model) handleToolsIngest(msg toolsIngestMsg) (tea.Model, tea.Cmd) {
	m.importBusy = false
	m.ingestJob = nil
	if !m.importing {
		return m, nil
	}
	if msg.err != nil {
		m.status = "Import failed: " + msg.err.Error()
		return m, nil
	}
	m.workspace = msg.ws
	m.search = searchsvc.New(m.workspace.Records)
	m.attachReferences()
	m.persistWorkspace()
	m.ensureBrowserSelection()
	m.refreshResults()
	for index, source := range m.workspace.Sources {
		if source.ID == msg.report.SourceID {
			m.importSourceCursor = index
			m.importFocus = "sources"
			break
		}
	}
	m.status = fmt.Sprintf("Imported %s (%s): %d records, %d prep, %d links",
		msg.report.Title, msg.report.Kind, msg.report.Records, msg.report.Planned, msg.report.Linked)
	if msg.report.Reference {
		m.status = fmt.Sprintf("Cached %s as reference · enable it on Sources to read and @ peek",
			msg.report.Title)
	}
	return m, nil
}
