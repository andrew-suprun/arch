package ui

import (
	"arch/device"
	"arch/files"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type model struct {
	paths            []string
	scanStates       []*files.ScanState
	locations        []location
	scanResults      []*files.ArchiveInfo
	maps             []maps   // source, copy1, copy2, ...
	links            []*links // copy1, copy2, ...
	screenSize       Size
	archiveViewLines int
	ctx              *Context
	lastMouseEvent   device.MouseEvent
	sortColumn       sortColumn
	sortAscending    []bool
}

func Run(dev device.Device, fs files.FS, paths []string) {
	m := &model{
		paths:         paths,
		scanStates:    make([]*files.ScanState, len(paths)),
		scanResults:   make([]*files.ArchiveInfo, len(paths)),
		ctx:           &Context{Device: dev, Style: defaultStyle},
		sortAscending: []bool{true, true, true, true},
	}

	fsEvents := make(chan files.Event)
	for _, archive := range paths {
		go func(archive string) {
			for ev := range fs.Scan(archive) {
				fsEvents <- ev
			}
		}(archive)
	}
	deviceEvents := make(chan device.Event)
	go func() {
		for {
			deviceEvents <- dev.PollEvent()
		}
	}()

	running := true
	for running {
		select {
		case fsEvent := <-fsEvents:
			m.handleFilesEvent(fsEvent)
		case deviceEvent := <-deviceEvents:
			running = m.handleDeviceEvent(deviceEvent)
		}
		m.ctx.Reset()
		Column(0,
			m.title(),
			m.scanStats(),
			m.treeView(),
			m.statusLine(),
		).Render(m.ctx, Position{0, 0}, m.screenSize)
		m.ctx.Device.Render()
	}

	fs.Stop()
	dev.Stop()
}

type fileInfo struct {
	kind    fileKind
	status  fileStatus
	archive string
	path    string
	name    string
	size    int
	modTime time.Time
	hash    string
	files   []*fileInfo
}

type location struct {
	file       *fileInfo
	selected   *fileInfo
	lineOffset int
}

type fileKind int

const (
	regularFile fileKind = iota
	folder
)

type fileStatus int

const (
	identical fileStatus = iota
	sourceOnly
	extraCopy
	copyOnly
	discrepancy // расхождение
)

type links struct {
	sourceLinks  map[*files.FileInfo]*files.FileInfo
	reverseLinks map[*files.FileInfo]*files.FileInfo
}

type maps struct {
	byName groupByName
	byHash groupByHash
}

type groupByName map[string]*files.FileInfo
type groupByHash map[string]files.FileInfos

var (
	defaultStyle       = device.Style{FG: 231, BG: 17}
	styleAppTitle      = device.Style{FG: 226, BG: 0, Flags: device.Bold + device.Italic}
	styleStatusLine    = device.Style{FG: 226, BG: 0}
	styleProgressBar   = device.Style{FG: 231, BG: 19}
	styleArchiveHeader = device.Style{FG: 231, BG: 8, Flags: device.Bold}
	styleBreadcrumbs   = device.Style{FG: 226, BG: 18, Flags: device.Bold + device.Italic}
)

type selectFile *fileInfo
type selectFolder *fileInfo

func statusColor(status fileStatus) byte {
	switch status {
	case identical:
		return 250
	case sourceOnly:
		return 82
	case extraCopy:
		return 226
	case copyOnly:
		return 214
	case discrepancy:
		return 196
	}
	return 231
}

func (s fileStatus) String() string {
	switch s {
	case identical:
		return "identical"
	case sourceOnly:
		return "sourceOnly"
	case copyOnly:
		return "copyOnly"
	case extraCopy:
		return "extraCopy"
	case discrepancy:
		return "discrepancy"
	}
	return "UNDEFINED"
}

func (s fileStatus) Merge(other fileStatus) fileStatus {
	if s > other {
		return s
	}
	return other
}

func (m *model) handleFilesEvent(event files.Event) {
	switch event := event.(type) {
	case *files.ScanState:
		for i := range m.paths {
			if m.paths[i] == event.Archive {
				m.scanStates[i] = event
				break
			}
		}

	case *files.ArchiveInfo:
		for i := range m.paths {
			if m.paths[i] == event.Archive {
				m.scanStates[i] = nil
				m.scanResults[i] = event
				break
			}
		}
		doneScanning := true
		for i := range m.paths {
			if m.scanResults[i] == nil {
				doneScanning = false
				break
			}
		}
		if doneScanning {
			m.analizeArchives()
		}

	default:
		log.Panicf("### unhandled files event %#v", event)
	}
}

func (m *model) handleDeviceEvent(event device.Event) bool {
	switch event := event.(type) {
	case device.ResizeEvent:
		m.screenSize = Size(event)

	case device.KeyEvent:
		if event.Name == "Ctrl+C" {
			return false
		}
		return m.handleKeyEvent(event)

	case device.MouseEvent:
		m.handleMouseEvent(event)

	case device.ScrollEvent:
		if event.Direction == device.ScrollUp {
			m.up()
		} else {
			m.down()
		}

	default:
		log.Panicf("### unhandled device event %#v", event)
	}
	return true
}

func (m *model) handleKeyEvent(key device.KeyEvent) bool {
	if key.Name == "Ctrl+C" {
		return false
	}

	loc := m.currentLocation()

	switch key.Name {
	case "Enter":
		m.enter()

	case "Esc":
		m.esc()

	case "Rune[R]", "Rune[r]":
		exec.Command("open", "-R", loc.selected.path).Start()

	case "Home":
		loc.selected = loc.file.files[0]

	case "End":
		loc.selected = loc.file.files[len(loc.file.files)-1]

	case "PgUp":
		loc.lineOffset -= m.archiveViewLines
		if loc.lineOffset < 0 {
			loc.lineOffset = 0
		}
		idxSelected := 0
		foundSelected := false
		for i := 0; i < len(loc.file.files); i++ {
			if loc.file.files[i] == loc.selected {
				idxSelected = i
				foundSelected = true
				break
			}
		}
		if foundSelected {
			idxSelected -= m.archiveViewLines
			if idxSelected < 0 {
				idxSelected = 0
			}
			loc.selected = loc.file.files[idxSelected]
		}

	case "PgDn":
		loc.lineOffset += m.archiveViewLines
		if loc.lineOffset > len(loc.file.files)-m.archiveViewLines {
			loc.lineOffset = len(loc.file.files) - m.archiveViewLines
		}
		idxSelected := 0
		foundSelected := false
		for i := 0; i < len(loc.file.files); i++ {
			if loc.file.files[i] == loc.selected {
				idxSelected = i
				foundSelected = true
				break
			}
		}
		if foundSelected {
			idxSelected += m.archiveViewLines
			if idxSelected > len(loc.file.files)-1 {
				idxSelected = len(loc.file.files) - 1
			}
			loc.selected = loc.file.files[idxSelected]
		}

	case "Up":
		m.up()

	case "Down":
		m.down()
	}
	return true
}

func (m *model) handleMouseEvent(event device.MouseEvent) {
	for _, target := range m.ctx.MouseTargetAreas {
		if target.Pos.X <= event.X && target.Pos.X+target.Size.Width > event.X &&
			target.Pos.Y <= event.Y && target.Pos.Y+target.Size.Height > event.Y {

			switch cmd := target.Command.(type) {
			case selectFolder:
				for i, loc := range m.locations {
					if loc.file == cmd && i < len(m.locations) {
						m.locations = m.locations[:i+1]
						return
					}
				}
			case selectFile:
				m.currentLocation().selected = cmd
				last := m.lastMouseEvent
				if event.Time.Sub(last.Time).Seconds() < 0.5 {
					m.enter()
				}
				m.lastMouseEvent = event
			case sortColumn:
				if cmd == m.sortColumn {
					m.sortAscending[m.sortColumn] = !m.sortAscending[m.sortColumn]
				} else {
					m.sortColumn = cmd
				}
				m.sort()
			}
		}
	}
}

func (m *model) analizeArchives() {
	m.scanStates = nil
	m.maps = make([]maps, len(m.scanResults))
	for i, scan := range m.scanResults {
		m.maps[i] = maps{
			byName: byName(scan.Files),
			byHash: byHash(scan.Files),
		}
	}

	m.links = make([]*links, len(m.scanResults)-1)
	for i, copy := range m.scanResults[1:] {
		m.links[i] = m.linkArchives(copy.Files)
	}
	m.buildFileTree()
}

func byName(infos files.FileInfos) groupByName {
	result := groupByName{}
	for _, info := range infos {
		result[info.Name] = info
	}
	return result
}

func byHash(archive files.FileInfos) groupByHash {
	result := groupByHash{}
	for _, info := range archive {
		result[info.Hash] = append(result[info.Hash], info)
	}
	return result
}

func (m *model) buildFileTree() {
	m.locations = []location{{
		file: &fileInfo{name: " Архив", kind: folder},
	}}

	uniqueFileNames := map[string]struct{}{}
	for _, info := range m.scanResults[0].Files {
		uniqueFileNames[info.Name] = struct{}{}
	}
	for i, copyScan := range m.scanResults[1:] {
		reverseLinks := m.links[i].reverseLinks
		for _, info := range copyScan.Files {
			if _, ok := reverseLinks[info]; !ok {
				uniqueFileNames[info.Name] = struct{}{}
			}
		}
	}

	for fullName := range uniqueFileNames {
		path := strings.Split(fullName, "/")
		name := path[len(path)-1]
		path = path[:len(path)-1]
		infos := make([]*files.FileInfo, len(m.maps))
		for i, info := range m.maps {
			infos[i] = info.byName[fullName]
		}
		for i, info := range infos {
			current := m.locations[0].file
			fileStack := []*fileInfo{current}
			if info == nil {
				continue
			}
			if i > 0 && infos[0] != nil && infos[0].Hash == info.Hash {
				continue
			}
			if i == 0 {
				current.size += info.Size
			}
			for pathIdx, dir := range path {
				sub := subFolder(current, dir)
				if i == 0 {
					sub.size += info.Size
				}
				if sub.archive == "" {
					sub.archive = info.Archive
					sub.path = filepath.Join(path[:pathIdx]...)
				}
				if sub.modTime.Before(info.ModTime) {
					sub.modTime = info.ModTime
				}
				current = sub
				fileStack = append(fileStack, current)
			}

			status := identical
			if i == 0 {
				for _, links := range m.links {
					if links.sourceLinks[info] == nil {
						status = sourceOnly
					}
				}
			} else {
				if i > 0 && infos[0] != nil {
					status = discrepancy
				} else {
					status = copyOnly
				}
			}

			currentFile := &fileInfo{
				kind:    regularFile,
				status:  status,
				archive: info.Archive,
				path:    filepath.Dir(info.Name),
				name:    name,
				size:    info.Size,
				modTime: info.ModTime,
				hash:    info.Hash,
			}
			current.files = append(current.files, currentFile)
			for _, current = range fileStack {
				current.status = status.Merge(current.status)
			}
		}
	}
	m.sort()
	PrintArchive(m.currentLocation().file, "")
}

func subFolder(dir *fileInfo, name string) *fileInfo {
	for i := range dir.files {
		if name == dir.files[i].name && dir.files[i].kind == folder {
			return dir.files[i]
		}
	}
	subFolder := &fileInfo{kind: folder, name: name}
	dir.files = append(dir.files, subFolder)
	return subFolder
}

func (a *links) String() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "Source Map:")
	for s, c := range a.sourceLinks {
		fmt.Fprintf(b, "  %s -> %s %s\n", s.Name, c.Name, s.Hash)
	}
	fmt.Fprintln(b, "Reverse Map:")
	for s, c := range a.reverseLinks {
		fmt.Fprintf(b, "  %s -> %s %s\n", s.Name, c.Name, s.Hash)
	}
	return b.String()
}

func (m *model) linkArchives(copyInfos files.FileInfos) *links {
	result := &links{
		sourceLinks:  map[*files.FileInfo]*files.FileInfo{},
		reverseLinks: map[*files.FileInfo]*files.FileInfo{},
	}
	for _, copy := range copyInfos {
		if sources, ok := m.maps[0].byHash[copy.Hash]; ok {
			match(sources, copy, result.sourceLinks)
		}
	}

	for source, copy := range result.sourceLinks {
		result.reverseLinks[copy] = source
	}

	return result
}

func match(sources files.FileInfos, copy *files.FileInfo, sourceMap map[*files.FileInfo]*files.FileInfo) *files.FileInfo {
	for _, source := range sources {
		if copy.Name == source.Name {
			sourceMap[source] = copy
			return nil
		}
	}

	for _, source := range sources {
		tmpCopy := sourceMap[source]
		sourceBase := filepath.Base(source.Name)
		if filepath.Base(copy.Name) == sourceBase && (tmpCopy == nil || filepath.Base(tmpCopy.Name) != sourceBase) {
			sourceMap[source] = copy
			copy = tmpCopy
			break
		}
	}

	if copy == nil {
		return nil
	}

	for _, source := range sources {
		tmpCopy := sourceMap[source]
		sourceBase := filepath.Base(source.Name)
		sourceDir := filepath.Dir(source.Name)
		if filepath.Dir(copy.Name) == sourceDir &&
			(tmpCopy == nil ||
				(filepath.Base(tmpCopy.Name) != sourceBase && filepath.Dir(tmpCopy.Name) != sourceDir)) {

			sourceMap[source] = copy
			copy = tmpCopy
			break
		}
	}

	if copy == nil {
		return nil
	}

	for _, source := range sources {
		if sourceMap[source] == nil {
			sourceMap[source] = copy
			return nil
		}
	}

	return copy
}

func PrintArchive(archive *fileInfo, prefix string) {
	kind := "D"
	if archive.kind == regularFile {
		kind = "F"
	}
	if archive.kind == regularFile {
		log.Printf("%s%s: %s status=%v size=%v hash=%v", prefix, kind, archive.name, archive.status, archive.size, archive.hash)
	} else {
		log.Printf("%s%s: %s status=%v size=%v", prefix, kind, archive.name, archive.status, archive.size)
	}
	for _, file := range archive.files {
		PrintArchive(file, prefix+"│ ")
	}
}

func (m *model) enter() {
	loc := m.currentLocation()
	if loc.selected != nil && loc.selected.kind == folder {
		m.locations = append(m.locations, location{file: loc.selected})
		m.sort()
	} else {
		fileName := filepath.Join(loc.selected.archive, loc.selected.path, loc.selected.name)
		exec.Command("open", fileName).Start()
	}
}

func (m *model) esc() {
	if len(m.locations) > 1 {
		m.locations = m.locations[:len(m.locations)-1]
		m.sort()
	}
}

func (m *model) up() {
	loc := m.currentLocation()
	if loc.selected != nil {
		for i, file := range loc.file.files {
			if file == loc.selected && i > 0 {
				loc.selected = loc.file.files[i-1]
				break
			}
		}
	} else {
		loc.selected = loc.file.files[len(loc.file.files)-1]
	}
}

func (m *model) down() {
	loc := m.currentLocation()
	if loc.selected != nil {
		for i, file := range loc.file.files {
			if file == loc.selected && i+1 < len(loc.file.files) {
				loc.selected = loc.file.files[i+1]
				break
			}
		}
	} else {
		loc.selected = loc.file.files[0]
	}
}

func (m *model) currentLocation() *location {
	if len(m.locations) == 0 {
		return nil
	}
	return &m.locations[len(m.locations)-1]
}

func (m *model) title() Widget {
	return Row(
		Styled(styleAppTitle, Text(" АРХИВАТОР").Flex(1)),
	)
}

func (m *model) statusLine() Widget {
	return Row(
		Styled(styleStatusLine, Text(" Status line will be here...").Flex(1)),
	)
}

func (m *model) scanStats() Widget {
	if m.scanStates == nil {
		return NullWidget{}
	}
	forms := []Widget{}
	first := true
	for i := range m.scanStates {
		if m.scanStates[i] != nil {
			if !first {
				forms = append(forms, Row(Text("").Flex(1).Pad('─')))
			}
			forms = append(forms, scanStatsForm(m.scanStates[i]))
			first = false
		}
	}
	forms = append(forms, Spacer{})
	return Column(1, forms...)
}

func scanStatsForm(state *files.ScanState) Widget {
	log.Println(Text(filepath.Base(state.Name)).Flex(1))
	return Column(0,
		Row(Text(" Архив                       "), Text(state.Archive).Flex(1), Text(" ")),
		Row(Text(" Каталог                     "), Text(filepath.Dir(state.Name)).Flex(1), Text(" ")),
		Row(Text(" Документ                    "), Text(filepath.Base(state.Name)).Flex(1), Text(" ")),
		Row(Text(" Ожидаемое Время Завершения  "), Text(time.Now().Add(state.Remaining).Format(time.TimeOnly)).Flex(1), Text(" ")),
		Row(Text(" Время До Завершения         "), Text(state.Remaining.Truncate(time.Second).String()).Flex(1), Text(" ")),
		Row(Text(" Общий Прогресс              "), Styled(styleProgressBar, ProgressBar(state.Progress)), Text(" ")),
	)
}

func (m *model) treeView() Widget {
	if len(m.locations) == 0 {
		return NullWidget{}
	}

	return Column(1,
		m.breadcrumbs(),
		Styled(styleArchiveHeader,
			Row(
				MouseTarget(sortByStatus, Text(" Статус"+m.sortIndicator(sortByStatus)).Width(9)),
				MouseTarget(sortByName, Text("  Документ"+m.sortIndicator(sortByName)).Width(20).Flex(1)),
				MouseTarget(sortByTime, Text("  Время Изменения"+m.sortIndicator(sortByTime)).Width(19)),
				MouseTarget(sortBySize, Text(fmt.Sprintf("%22s", "Размер"+m.sortIndicator(sortBySize)+" "))),
			),
		),
		Scroll(nil, Constraint{Size{0, 0}, Flex{1, 1}},
			func(size Size) Widget {
				m.archiveViewLines = size.Height
				location := m.currentLocation()
				if location.lineOffset > len(location.file.files)+1-size.Height {
					location.lineOffset = len(location.file.files) + 1 - size.Height
				}
				if location.lineOffset < 0 {
					location.lineOffset = 0
				}
				if location.selected != nil {
					idx := -1
					for i := range location.file.files {
						if location.selected == location.file.files[i] {
							idx = i
							break
						}
					}
					if idx >= 0 {
						if location.lineOffset > idx {
							location.lineOffset = idx
						}
						if location.lineOffset < idx+1-size.Height {
							location.lineOffset = idx + 1 - size.Height
						}
					}
				}
				rows := []Widget{}
				i := 0
				var file *fileInfo
				for i, file = range location.file.files[location.lineOffset:] {
					if i >= size.Height {
						break
					}
					rows = append(rows, Styled(styleFile(file, location.selected == file),
						MouseTarget(selectFile(file), Row(
							Text(" "+file.status.String()).Width(9),
							Text("  "),
							Text(displayName(file)).Width(20).Flex(1),
							Text("  "),
							Text(file.modTime.Format(time.DateTime)),
							Text("  "),
							Text(formatSize(file.size)).Width(18),
						)),
					))
				}
				rows = append(rows, Spacer{})
				return Column(0, rows...)
			},
		),
	)
}

func displayName(file *fileInfo) string {
	if file.kind == folder {
		return "▶ " + file.name
	}
	return "  " + file.name
}

func (m *model) sortIndicator(column sortColumn) string {
	if column == m.sortColumn {
		if m.sortAscending[column] {
			return " ▲"
		}
		return " ▼"
	}
	return ""
}

func (m *model) breadcrumbs() Widget {
	widgets := make([]Widget, 0, len(m.locations)*2)
	for i, loc := range m.locations {
		if i > 0 {
			widgets = append(widgets, Text(" / "))
		}
		widgets = append(widgets,
			MouseTarget(selectFolder(loc.file),
				Styled(styleBreadcrumbs, Text(loc.file.name)),
			),
		)
	}
	widgets = append(widgets, Spacer{})
	return Row(widgets...)
}

func styleFile(file *fileInfo, selected bool) device.Style {
	bg, flags := byte(17), device.Flags(0)
	if file.kind == folder {
		bg, flags = byte(18), device.Bold
	}
	result := device.Style{FG: statusColor(file.status), BG: bg, Flags: flags}
	if selected {
		result.Flags |= device.Reverse
	}
	return result
}

func formatSize(size int) string {
	str := fmt.Sprintf("%13d ", size)
	slice := []string{str[:1], str[1:4], str[4:7], str[7:10]}
	b := strings.Builder{}
	for _, s := range slice {
		b.WriteString(s)
		if s == " " || s == "   " {
			b.WriteString(" ")
		} else {
			b.WriteString(",")
		}
	}
	b.WriteString(str[10:])
	return b.String()
}
