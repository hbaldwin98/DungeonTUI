package domain

import (
	"sort"
	"strings"
)

type RecordTreeKind string

const (
	RecordTreeFolder RecordTreeKind = "folder"
	RecordTreeRecord RecordTreeKind = "record"
)

// RecordTreeRow is one visible row in a typed wiki list: a collapsible
// source/type folder or a record leaf.
type RecordTreeRow struct {
	Kind   RecordTreeKind
	Path   string
	Depth  int
	Label  string
	Count  int
	Record Record
}

// RecordTypeFolder is the leaf folder name for an imported record.
func RecordTypeFolder(record Record) string {
	for _, tag := range record.Tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "spell":
			return "Spells"
		case "class", "subclass":
			return "Classes"
		case "feat":
			return "Feats"
		case "species":
			return "Species"
		case "background":
			return "Backgrounds"
		case "condition":
			return "Conditions"
		case "chapter":
			return "Chapters"
		}
	}
	switch record.Type {
	case Creature:
		return "Creatures"
	case NPC:
		return "NPCs"
	case Item:
		return "Items"
	case Location:
		return "Locations"
	case Note:
		return "Notes"
	case Rule:
		return "Rules"
	case Character:
		return "Characters"
	case Faction:
		return "Factions"
	default:
		if record.Type == "" {
			return "Records"
		}
		return string(record.Type)
	}
}

// SourceRecordFolder files an imported record under its book, then type.
func SourceRecordFolder(sourceTitle string, record Record) string {
	title := strings.TrimSpace(sourceTitle)
	if title == "" {
		title = strings.TrimSpace(record.Source)
	}
	if title == "" {
		title = record.SourceID
	}
	return NormalizeFolder(title + "/" + RecordTypeFolder(record))
}

// RecordFolderPath is the display folder for a wiki record. Stored Folder
// wins; otherwise imported records derive Source/Type so older workspaces
// still group after a 5e.tools ingest.
func RecordFolderPath(record Record, sources []SourceDocument) string {
	if folder := NormalizeFolder(record.Folder); folder != "" {
		return folder
	}
	if record.SourceID == "" {
		return ""
	}
	title := strings.TrimSpace(record.Source)
	for _, source := range sources {
		if source.ID == record.SourceID {
			if strings.TrimSpace(source.Title) != "" {
				title = source.Title
			}
			break
		}
	}
	return SourceRecordFolder(title, record)
}

type recordFolderNode struct {
	name     string
	path     string
	children map[string]*recordFolderNode
	order    []string
	records  []Record
	labels   map[string]string
}

func (n *recordFolderNode) child(name, path string) *recordFolderNode {
	if n.children == nil {
		n.children = map[string]*recordFolderNode{}
	}
	if child, ok := n.children[name]; ok {
		return child
	}
	child := &recordFolderNode{name: name, path: path, children: map[string]*recordFolderNode{}}
	n.children[name] = child
	n.order = append(n.order, name)
	return child
}

func (n *recordFolderNode) recordCount() int {
	total := len(n.records)
	for _, name := range n.order {
		total += n.children[name].recordCount()
	}
	return total
}

// FlattenRecordTree builds visible folder/record rows. collapsed reports
// whether a folder path should hide its children. Campaign records with an
// empty folder render first as loose leaves.
func FlattenRecordTree(records []Record, sources []SourceDocument, collapsed func(string) bool) []RecordTreeRow {
	sorted := append([]Record(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return RecordListLess(sorted[i].Title, sorted[j].Title)
	})

	type placed struct {
		record Record
		path   string
		label  string
	}
	items := make([]placed, 0, len(sorted))
	groups := map[string]bool{}
	for _, record := range sorted {
		base := RecordFolderPath(record, sources)
		path := base
		label := record.Title
		if group, leaf, ok := SplitRelatedTitle(record.Title); ok {
			path = NormalizeFolder(base + "/" + group)
			label = leaf
			groups[strings.ToLower(base+"/"+group)] = true
		}
		items = append(items, placed{record: record, path: path, label: label})
	}
	for i, item := range items {
		if item.path == "" {
			continue
		}
		if _, leaf, ok := SplitRelatedTitle(item.record.Title); ok && leaf != "" {
			continue
		}
		base := RecordFolderPath(item.record, sources)
		key := strings.ToLower(base + "/" + item.record.Title)
		if groups[key] {
			items[i].path = NormalizeFolder(base + "/" + item.record.Title)
		}
	}

	root := &recordFolderNode{children: map[string]*recordFolderNode{}}
	loose := make([]Record, 0)
	for _, item := range items {
		if item.path == "" {
			loose = append(loose, item.record)
			continue
		}
		node := root
		acc := ""
		for _, part := range strings.Split(item.path, "/") {
			if acc == "" {
				acc = part
			} else {
				acc += "/" + part
			}
			node = node.child(part, acc)
		}
		node.records = append(node.records, item.record)
		if node.labels == nil {
			node.labels = map[string]string{}
		}
		node.labels[item.record.ID] = item.label
	}

	rows := make([]RecordTreeRow, 0, len(sorted)+8)
	for _, record := range loose {
		rows = append(rows, RecordTreeRow{
			Kind:   RecordTreeRecord,
			Depth:  0,
			Label:  record.Title,
			Record: record,
		})
	}
	walkRecordFolder(root, collapsed, &rows)
	return rows
}

func walkRecordFolder(node *recordFolderNode, collapsed func(string) bool, rows *[]RecordTreeRow) {
	names := append([]string(nil), node.order...)
	sort.SliceStable(names, func(i, j int) bool {
		return RecordListLess(names[i], names[j])
	})
	hide := collapsed
	if hide == nil {
		hide = func(string) bool { return false }
	}
	for _, name := range names {
		child := node.children[name]
		depth := 0
		if child.path != "" {
			depth = strings.Count(child.path, "/")
		}
		*rows = append(*rows, RecordTreeRow{
			Kind:  RecordTreeFolder,
			Path:  child.path,
			Depth: depth,
			Label: child.name,
			Count: child.recordCount(),
		})
		if hide(child.path) {
			continue
		}
		walkRecordFolder(child, hide, rows)
		recs := append([]Record(nil), child.records...)
		sort.SliceStable(recs, func(i, j int) bool {
			return RecordListLess(recordTreeLabel(child, recs[i]), recordTreeLabel(child, recs[j]))
		})
		for _, record := range recs {
			*rows = append(*rows, RecordTreeRow{
				Kind:   RecordTreeRecord,
				Path:   child.path,
				Depth:  depth + 1,
				Label:  recordTreeLabel(child, record),
				Record: record,
			})
		}
	}
}

func recordTreeLabel(node *recordFolderNode, record Record) string {
	if node != nil && node.labels != nil {
		if label := node.labels[record.ID]; label != "" {
			return label
		}
	}
	return record.Title
}

// SplitRelatedTitle pulls "Millhaven — 1. The Mill Inn" into a parent group
// and a short leaf so already-imported adventures nest without a re-ingest.
func SplitRelatedTitle(title string) (group, leaf string, ok bool) {
	title = strings.TrimSpace(title)
	for _, sep := range []string{" — ", " – ", " - "} {
		parent, rest, found := strings.Cut(title, sep)
		if !found {
			continue
		}
		parent = strings.TrimSpace(parent)
		rest = strings.TrimSpace(rest)
		if parent == "" || rest == "" || !relatedChildTitle(rest) {
			continue
		}
		return parent, rest, true
	}
	return "", "", false
}

func relatedChildTitle(leaf string) bool {
	i := 0
	for i < len(leaf) && leaf[i] >= '0' && leaf[i] <= '9' {
		i++
	}
	if i == 0 {
		return false
	}
	if i == len(leaf) {
		return true
	}
	return leaf[i] == '.' || leaf[i] == ' '
}

// RecordListLess sorts wiki rows: unnumbered titles first (site overview),
// then numbered rooms in numeric order, then remaining names.
func RecordListLess(a, b string) bool {
	aNum, aHas := leadingTitleNumber(a)
	bNum, bHas := leadingTitleNumber(b)
	if aHas && bHas && aNum != bNum {
		return aNum < bNum
	}
	if aHas != bHas {
		return !aHas
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

func leadingTitleNumber(title string) (int, bool) {
	title = strings.TrimSpace(title)
	n := 0
	i := 0
	for i < len(title) && title[i] >= '0' && title[i] <= '9' {
		n = n*10 + int(title[i]-'0')
		i++
	}
	if i == 0 {
		return 0, false
	}
	return n, true
}

// FolderParent splits a slash path into the parent prefix and last segment.
func FolderParent(path string) (parent, leaf string) {
	path = NormalizeFolder(path)
	if path == "" {
		return "", ""
	}
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}

// NestOverviewFolders moves a parent site into the same folder as its rooms
// once children have been filed under Source/Type/Site.
func NestOverviewFolders(records []Record) {
	used := map[string]string{}
	for _, record := range records {
		parent, leaf := FolderParent(record.Folder)
		if leaf == "" || strings.EqualFold(leaf, record.Title) {
			continue
		}
		used[strings.ToLower(leaf)+"|"+strings.ToLower(parent)] = parent
	}
	for i, record := range records {
		folder := NormalizeFolder(record.Folder)
		key := strings.ToLower(record.Title) + "|" + strings.ToLower(folder)
		if base, ok := used[key]; ok {
			records[i].Folder = NormalizeFolder(base + "/" + record.Title)
		}
	}
}

func SourceFolderWithGroup(sourceTitle string, record Record, group string) string {
	base := SourceRecordFolder(sourceTitle, record)
	group = NormalizeFolder(group)
	if group == "" {
		return base
	}
	return NormalizeFolder(base + "/" + group)
}
