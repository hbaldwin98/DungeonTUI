package domain

import (
	"sort"
	"strings"
	"time"
)

const UnfiledFolder = "Unfiled"

type SessionTreeKind string

const (
	SessionTreeFolder  SessionTreeKind = "folder"
	SessionTreeSession SessionTreeKind = "session"
)

// SessionTreeRow is one visible row in the Sessions list: a collapsible
// folder header or a sit leaf.
type SessionTreeRow struct {
	Kind    SessionTreeKind
	Path    string
	Depth   int
	Label   string
	Count   int // descendant sits, folder rows only
	Session SessionRecord
}

// NormalizeFolder trims a slash path and drops empty, ".", and ".." segments.
func NormalizeFolder(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

// SessionFolderPath is the stored-or-derived folder a sit lives under.
// Named Folder wins; otherwise the sit buckets by started month (UTC), or
// Unfiled when it has no timestamp yet.
func SessionFolderPath(session SessionRecord) string {
	if folder := NormalizeFolder(session.Folder); folder != "" {
		return folder
	}
	if session.StartedAt.IsZero() {
		return UnfiledFolder
	}
	return session.StartedAt.UTC().Format("2006-01")
}

// FolderPrefix reports whether displayPath is path or a descendant of path.
func FolderPrefix(displayPath, path string) bool {
	path = NormalizeFolder(path)
	displayPath = NormalizeFolder(displayPath)
	if path == "" || displayPath == "" {
		return false
	}
	return displayPath == path || strings.HasPrefix(displayPath, path+"/")
}

// ApplySessionFolder sets Folder on the matching sit IDs. An empty folder
// returns those sits to month / Unfiled grouping.
func ApplySessionFolder(sessions []SessionRecord, ids []string, folder string) []SessionRecord {
	folder = NormalizeFolder(folder)
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		want[id] = struct{}{}
	}
	if len(want) == 0 {
		return sessions
	}
	out := append([]SessionRecord(nil), sessions...)
	for index := range out {
		if _, ok := want[out[index].ID]; ok {
			out[index].Folder = folder
		}
	}
	return out
}

type folderNode struct {
	name     string
	path     string
	children map[string]*folderNode
	order    []string
	sessions []SessionRecord
}

func (n *folderNode) child(name, path string) *folderNode {
	if n.children == nil {
		n.children = map[string]*folderNode{}
	}
	if child, ok := n.children[name]; ok {
		return child
	}
	child := &folderNode{name: name, path: path, children: map[string]*folderNode{}}
	n.children[name] = child
	n.order = append(n.order, name)
	return child
}

func (n *folderNode) sessionCount() int {
	total := len(n.sessions)
	for _, name := range n.order {
		total += n.children[name].sessionCount()
	}
	return total
}

// FlattenSessionTree builds visible folder/sit rows. collapsed paths hide
// their descendant folders and sits. Sits sort newest-first within a folder.
func FlattenSessionTree(sessions []SessionRecord, collapsed map[string]bool) []SessionTreeRow {
	sorted := append([]SessionRecord(nil), sessions...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].StartedAt.After(sorted[j].StartedAt)
	})

	root := &folderNode{children: map[string]*folderNode{}}
	for _, session := range sorted {
		path := SessionFolderPath(session)
		node := root
		acc := ""
		for _, part := range strings.Split(path, "/") {
			if acc == "" {
				acc = part
			} else {
				acc += "/" + part
			}
			node = node.child(part, acc)
		}
		node.sessions = append(node.sessions, session)
	}

	rows := make([]SessionTreeRow, 0)
	walkFolderNode(root, collapsed, &rows)
	return rows
}

func walkFolderNode(node *folderNode, collapsed map[string]bool, rows *[]SessionTreeRow) {
	names := append([]string(nil), node.order...)
	sortFolderNames(names)
	for _, name := range names {
		child := node.children[name]
		depth := 0
		if child.path != "" {
			depth = strings.Count(child.path, "/")
		}
		*rows = append(*rows, SessionTreeRow{
			Kind:  SessionTreeFolder,
			Path:  child.path,
			Depth: depth,
			Label: child.name,
			Count: child.sessionCount(),
		})
		if collapsed[child.path] {
			continue
		}
		walkFolderNode(child, collapsed, rows)
		for _, session := range child.sessions {
			*rows = append(*rows, SessionTreeRow{
				Kind:    SessionTreeSession,
				Path:    child.path,
				Depth:   depth + 1,
				Label:   session.Title,
				Session: session,
			})
		}
	}
}

func sortFolderNames(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		return folderRank(names[i]) < folderRank(names[j]) ||
			(folderRank(names[i]) == folderRank(names[j]) && folderNameLess(names[i], names[j]))
	})
}

func folderRank(name string) int {
	if isMonthFolder(name) {
		return 1
	}
	if name == UnfiledFolder {
		return 2
	}
	return 0
}

func folderNameLess(a, b string) bool {
	if isMonthFolder(a) && isMonthFolder(b) {
		return a > b // newest month first
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

func isMonthFolder(name string) bool {
	if len(name) != 7 {
		return false
	}
	_, err := time.Parse("2006-01", name)
	return err == nil
}

func IsDerivedFolder(path string) bool {
	path = NormalizeFolder(path)
	if path == "" || path == UnfiledFolder {
		return true
	}
	for _, part := range strings.Split(path, "/") {
		if !isMonthFolder(part) {
			return false
		}
	}
	return true
}
