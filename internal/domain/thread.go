package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ThreadState is the owner-set lifecycle of a campaign thread.
type ThreadState string

const (
	ThreadOpen      ThreadState = "open"
	ThreadAdvancing ThreadState = "advancing"
	ThreadDormant   ThreadState = "dormant"
	ThreadResolved  ThreadState = "resolved"
)

// ThreadStates lists every state in urgency order.
var ThreadStates = []ThreadState{ThreadAdvancing, ThreadOpen, ThreadDormant, ThreadResolved}

// neglectedAfterSits is how many ended sits may pass without a thread coming
// up before an open or advancing thread counts as neglected.
const neglectedAfterSits = 2

// ParseThreadState accepts a state word case-insensitively.
func ParseThreadState(value string) (ThreadState, bool) {
	switch ThreadState(strings.ToLower(strings.TrimSpace(value))) {
	case ThreadOpen:
		return ThreadOpen, true
	case ThreadAdvancing:
		return ThreadAdvancing, true
	case ThreadDormant:
		return ThreadDormant, true
	case ThreadResolved:
		return ThreadResolved, true
	}
	return "", false
}

// EffectiveThreadState resolves a thread's state. An explicit state wins; a
// thread from before states existed falls back to a state-word tag, and
// otherwise reads as open, because a thread exists to track something
// unresolved.
func EffectiveThreadState(record Record) ThreadState {
	if state, ok := ParseThreadState(string(record.ThreadState)); ok {
		return state
	}
	for _, state := range ThreadStates {
		if recordHasTag(record, string(state)) {
			return state
		}
	}
	return ThreadOpen
}

func threadStateRank(state ThreadState) int {
	for index, candidate := range ThreadStates {
		if candidate == state {
			return index
		}
	}
	return len(ThreadStates)
}

// SetThreadState is the explicit owner transition. It changes only the state:
// body, citations, and session history stay intact, so a resolved thread
// still reads as the record of what happened.
func SetThreadState(record Record, state ThreadState) (Record, error) {
	if record.Type != Thread {
		return record, fmt.Errorf("%q is not a thread", record.Title)
	}
	if record.Authority == Proposal {
		return record, fmt.Errorf("accept the proposal %q before tracking its state", record.Title)
	}
	if _, ok := ParseThreadState(string(state)); !ok {
		return record, fmt.Errorf("unknown thread state %q", state)
	}
	record.ThreadState = state
	return record, nil
}

// ThreadSections are the Markdown body sections a thread may carry.
type ThreadSections struct {
	Stakes          string
	NextBeats       []string
	LastDevelopment string
}

// ParseThreadSections reads "## Stakes", "## Next beats", and
// "## Last development" from a thread body. Headings are matched
// case-insensitively and every other section is ignored, so a thread stays
// ordinary Markdown.
func ParseThreadSections(body string) ThreadSections {
	var sections ThreadSections
	current := ""
	var buffer []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(buffer, "\n"))
		switch current {
		case "stakes":
			sections.Stakes = text
		case "last development":
			sections.LastDevelopment = text
		case "next beats":
			for _, line := range buffer {
				item := strings.TrimSpace(line)
				item = strings.TrimSpace(strings.TrimLeft(item, "-*+"))
				if item != "" {
					sections.NextBeats = append(sections.NextBeats, item)
				}
			}
		}
		buffer = nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flush()
			current = strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			continue
		}
		buffer = append(buffer, line)
	}
	flush()
	return sections
}

// ThreadStatus is a thread's derived operational view.
type ThreadStatus struct {
	Record    Record
	State     ThreadState
	Stakes    string
	NextBeats []string
	Involved  []Record
	// LastDevelopment is the newest transcript line naming the thread, or the
	// owner's "## Last development" section when play never named it.
	LastDevelopment   string
	LastDevelopmentAt time.Time
	LastSessionID     string
	LastSession       string
	// QuietSits counts ended sits since the thread last came up in play.
	QuietSits   int
	NeverInPlay bool
	Neglected   bool
}

// Active reports whether a thread still needs attention.
func (t ThreadStatus) Active() bool {
	return t.State != ThreadResolved
}

// Attention renders the one-line reason a thread sorts where it does.
func (t ThreadStatus) Attention() string {
	parts := []string{string(t.State)}
	switch {
	case t.Neglected:
		parts = append(parts, fmt.Sprintf("quiet %d sits", t.QuietSits))
	case t.NeverInPlay && t.Active():
		parts = append(parts, "not yet in play")
	case t.LastSession != "":
		parts = append(parts, "last "+t.LastSession)
	}
	return strings.Join(parts, " · ")
}

// DeriveThreadStatus builds a thread's operational view from its record,
// linked sessions, and body sections.
func DeriveThreadStatus(workspace Workspace, record Record, scope Scope) ThreadStatus {
	sections := ParseThreadSections(record.Body)
	status := ThreadStatus{
		Record:          record,
		State:           EffectiveThreadState(record),
		Stakes:          sections.Stakes,
		NextBeats:       sections.NextBeats,
		LastDevelopment: sections.LastDevelopment,
	}
	resolved, _ := EntityOutgoingRefs(record, workspace.Records)
	seen := map[string]bool{}
	for _, mention := range resolved {
		if seen[mention.RecordID] {
			continue
		}
		if target, ok := findRecord(workspace.Records, mention.RecordID); ok && target.ID != record.ID {
			seen[target.ID] = true
			status.Involved = append(status.Involved, target)
		}
	}

	sessions := campaignSessions(workspace.Sessions, scope)
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].StartedAt.Before(sessions[j].StartedAt) })
	lastTouched := -1
	for index, session := range sessions {
		touched := session.HasLink(record.ID)
		for _, entry := range session.Entries {
			if entry.Undone || !entryLinks(entry, record.ID) {
				continue
			}
			touched = true
			status.LastDevelopment = strings.TrimSpace(entry.Text)
			status.LastDevelopmentAt = entry.CreatedAt
		}
		if touched {
			lastTouched = index
			status.LastSessionID = session.ID
			status.LastSession = session.Title
			if status.LastDevelopmentAt.IsZero() {
				status.LastDevelopmentAt = session.StartedAt
			}
		}
	}
	status.NeverInPlay = lastTouched < 0
	for index := lastTouched + 1; index < len(sessions); index++ {
		if sessions[index].EndedAt != nil {
			status.QuietSits++
		}
	}
	status.Neglected = !status.NeverInPlay && status.QuietSits >= neglectedAfterSits &&
		(status.State == ThreadOpen || status.State == ThreadAdvancing)
	return status
}

func entryLinks(entry TranscriptEntry, recordID string) bool {
	for _, link := range entry.Links {
		if link.RecordID == recordID {
			return true
		}
	}
	return false
}

// CampaignThreads lists the scope's owner-tracked threads by urgency:
// advancing, open, dormant, then resolved; within a state, neglected threads
// first, then threads not yet in play, then the least recently developed.
// AI proposals and superseded threads are never listed, so a generated beat
// cannot pass for a tracked thread.
func CampaignThreads(workspace Workspace, scope Scope, includeResolved bool) []ThreadStatus {
	enabled := workspace.EnabledSourceIDs(scope)
	var threads []ThreadStatus
	for _, record := range workspace.Records {
		if record.Type != Thread || record.Authority == Proposal || record.Authority == Superseded || record.IsAIContent {
			continue
		}
		if !RecordVisibleIn(record, scope, enabled) {
			continue
		}
		status := DeriveThreadStatus(workspace, record, scope)
		if !includeResolved && !status.Active() {
			continue
		}
		threads = append(threads, status)
	}
	sort.SliceStable(threads, func(i, j int) bool {
		a, b := threads[i], threads[j]
		if rankA, rankB := threadStateRank(a.State), threadStateRank(b.State); rankA != rankB {
			return rankA < rankB
		}
		if a.Neglected != b.Neglected {
			return a.Neglected
		}
		if a.QuietSits != b.QuietSits {
			return a.QuietSits > b.QuietSits
		}
		if a.NeverInPlay != b.NeverInPlay {
			return a.NeverInPlay
		}
		if !a.LastDevelopmentAt.Equal(b.LastDevelopmentAt) {
			return a.LastDevelopmentAt.Before(b.LastDevelopmentAt)
		}
		return a.Record.Title < b.Record.Title
	})
	return threads
}

// ThreadsTouching lists active threads that involve any of the given records,
// in urgency order. It is how prep and live play find the threads their cast
// and location carry.
func ThreadsTouching(workspace Workspace, scope Scope, recordIDs []string) []ThreadStatus {
	want := map[string]bool{}
	for _, id := range recordIDs {
		if id != "" {
			want[id] = true
		}
	}
	var out []ThreadStatus
	for _, thread := range CampaignThreads(workspace, scope, false) {
		if want[thread.Record.ID] {
			out = append(out, thread)
			continue
		}
		for _, involved := range thread.Involved {
			if want[involved.ID] {
				out = append(out, thread)
				break
			}
		}
	}
	return out
}
