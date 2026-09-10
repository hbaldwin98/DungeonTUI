package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

// JourneySizes are the terminal sizes every core journey must survive: the
// smallest common terminal, the harness default, and a wide desktop.
var JourneySizes = [][2]int{{80, 24}, {100, 30}, {160, 45}}

// JourneyStep is one owner action. Exactly one of Key or Type is set. Typing
// a phrase counts as a single action, because a DM types a thought in one go.
type JourneyStep struct {
	Key  string
	Type string
}

// Journey is a realistic DM task with an action budget. Setup puts the
// harness in the state the DM would already be in and is not counted.
type Journey struct {
	Name   string
	Goal   string
	Budget int
	Setup  func(*Harness) error
	Steps  []JourneyStep
	// Until repeats the step it is attached to (by index) until it reports
	// true, for steps whose count depends on layout, such as Tab cycling.
	Until map[int]func(*Harness) bool
	Check func(*Harness) error
	// ApprovesCanon is true only for journeys where the owner accepts review
	// items; every other journey must leave canon exactly as it found it.
	ApprovesCanon bool
}

// JourneyResult is one journey at one size.
type JourneyResult struct {
	Journey  string
	Width    int
	Height   int
	Actions  int
	Budget   int
	Elapsed  time.Duration
	Problems []string
	Final    *Harness
}

// OK reports whether the journey met its goal inside budget with no audit
// problems.
func (r JourneyResult) OK() bool { return len(r.Problems) == 0 }

// Summary is one scannable result line.
func (r JourneyResult) Summary() string {
	status := "ok"
	if !r.OK() {
		status = "FAIL " + strings.Join(r.Problems, "; ")
	}
	return fmt.Sprintf("%-18s %3dx%-3d actions %d/%d  %s", r.Journey, r.Width, r.Height, r.Actions, r.Budget, status)
}

// maxUntilRepeats bounds a repeated step so a broken journey fails instead of
// spinning.
const maxUntilRepeats = 8

// RunJourney drives one journey and audits every frame it produces.
func RunJourney(journey Journey, width, height int) JourneyResult {
	result := JourneyResult{Journey: journey.Name, Width: width, Height: height, Budget: journey.Budget}
	started := time.Now()
	h := NewHarness(width, height)
	h.Resize(width, height)
	result.Final = h
	if journey.Setup != nil {
		if err := journey.Setup(h); err != nil {
			result.Problems = append(result.Problems, "setup: "+err.Error())
			return result
		}
	}
	canon := canonSnapshot(h.Model.workspace)
	problems := map[string]bool{}
	note := func(problem string) {
		if !problems[problem] {
			problems[problem] = true
			result.Problems = append(result.Problems, problem)
		}
	}
	for index, step := range journey.Steps {
		repeats := 1
		for attempt := 0; attempt < repeats; attempt++ {
			if step.Key != "" {
				if !keyDocumented(h, step.Key) {
					note(fmt.Sprintf("undocumented key %q in %s", step.Key, h.Model.helpContextName()))
				}
				h.Key(step.Key)
			} else {
				h.Type(step.Type)
			}
			result.Actions++
			auditFrame(h.Frame(), note)
			if until, ok := journey.Until[index]; ok && !until(h) {
				if repeats < maxUntilRepeats {
					repeats++
				} else {
					note(fmt.Sprintf("step %d never reached its target", index+1))
				}
			}
		}
	}
	result.Elapsed = time.Since(started)
	if result.Actions > journey.Budget {
		note(fmt.Sprintf("over budget: %d actions for %d", result.Actions, journey.Budget))
	}
	if journey.Check != nil {
		if err := journey.Check(h); err != nil {
			note("goal: " + err.Error())
		}
	}
	if !journey.ApprovesCanon {
		for _, change := range canonChanges(canon, h.Model.workspace) {
			note("accidental canon write: " + change)
		}
	}
	return result
}

// auditFrame applies the checks every frame must pass.
func auditFrame(frame Frame, note func(string)) {
	if !frame.FillsTerminal() {
		note("frame does not fill terminal: " + strings.Join(frame.FillErrors(), ", "))
	}
	for _, code := range BackgroundCodes(frame.Raw) {
		note("background fill " + code)
	}
}

var sgrSequence = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// BackgroundCodes lists every SGR background parameter in raw output. The
// app paints no surfaces (decision #27), so any hit is a background artifact
// fighting the terminal's own default background.
func BackgroundCodes(raw string) []string {
	var found []string
	seen := map[string]bool{}
	for _, match := range sgrSequence.FindAllStringSubmatch(raw, -1) {
		params := strings.FieldsFunc(match[1], func(r rune) bool { return r == ';' || r == ':' })
		for index := 0; index < len(params); index++ {
			value, err := strconv.Atoi(params[index])
			if err != nil {
				continue
			}
			switch {
			case value == 38 || value == 58:
				// Foreground/underline colour: skip its arguments so a
				// truecolor component such as 48 is not misread.
				index += colorArgs(params, index)
				continue
			case value == 48 || (value >= 40 && value <= 47) || (value >= 100 && value <= 107):
				if !seen[match[0]] {
					seen[match[0]] = true
					found = append(found, strconv.Quote(match[0]))
				}
				index += colorArgs(params, index)
			}
		}
	}
	return found
}

func colorArgs(params []string, index int) int {
	if index+1 >= len(params) {
		return 0
	}
	switch params[index+1] {
	case "5":
		return 2
	case "2":
		return 4
	}
	return 0
}

// keySpellings maps harness key names to how help and footers write them.
var keySpellings = map[string][]string{
	"enter": {"Enter"}, "esc": {"Esc"}, "tab": {"Tab"}, "backspace": {"Backspace"},
	"right": {"→"}, "left": {"←"}, "up": {"↑"}, "down": {"↓"},
	"pgup": {"PgUp"}, "pgdown": {"PgDn"},
	"ctrl+n": {"Ctrl+N"}, "ctrl+e": {"Ctrl+E"}, "ctrl+s": {"Ctrl+S"},
	"alt+left": {"Alt+←"}, "alt+right": {"Alt+→"},
}

// keyDocumented reports whether a key is named in the help reference or the
// visible footer for the context it is pressed in. A journey that needs a key
// the screen never mentions is a discoverability regression.
func keyDocumented(h *Harness, key string) bool {
	var text strings.Builder
	for _, section := range h.Model.helpSections() {
		text.WriteString(strings.Join(section, " · "))
		text.WriteString("\n")
	}
	frame := h.Frame()
	text.WriteString(frame.Lines[len(frame.Lines)-1])
	text.WriteString("\n")
	// Inline hints inside panes count too: they are on screen where the key
	// is used.
	text.WriteString(frame.Plain)
	spellings, ok := keySpellings[key]
	if !ok {
		spellings = []string{key}
	}
	for _, spelling := range spellings {
		if containsToken(text.String(), spelling) {
			return true
		}
	}
	return false
}

func containsToken(text, token string) bool {
	boundary := func(r rune) bool {
		return r == ' ' || r == '/' || r == '·' || r == '\n' || r == '(' || r == ')' || r == ','
	}
	for start := 0; ; {
		at := strings.Index(text[start:], token)
		if at < 0 {
			return false
		}
		at += start
		end := at + len(token)
		before := at == 0 || boundary(lastRune(text[:at]))
		after := end == len(text) || boundary(firstRune(text[end:]))
		if before && after {
			return true
		}
		start = at + 1
	}
}

func lastRune(s string) rune {
	runes := []rune(s)
	return runes[len(runes)-1]
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// helpContextName names the context for a problem report.
func (m Model) helpContextName() string {
	sections := m.helpSections()
	if len(sections) == 0 || len(sections[0]) == 0 {
		return "unknown context"
	}
	return sections[0][0]
}

type canonState struct {
	authority domain.Authority
	body      string
}

func canonSnapshot(workspace domain.Workspace) map[string]canonState {
	out := map[string]canonState{}
	for _, record := range workspace.Records {
		out[record.ID] = canonState{authority: record.Authority, body: record.Body}
	}
	return out
}

// canonChanges lists records that became canon or whose canon text changed.
func canonChanges(before map[string]canonState, workspace domain.Workspace) []string {
	var changes []string
	for _, record := range workspace.Records {
		if record.Authority != domain.Canon {
			continue
		}
		previous, existed := before[record.ID]
		switch {
		case !existed:
			changes = append(changes, "new canon record "+record.Title)
		case previous.authority != domain.Canon:
			changes = append(changes, record.Title+" became canon")
		case previous.body != record.Body:
			changes = append(changes, record.Title+" canon text changed")
		}
	}
	return changes
}

func findRecordByTitle(workspace domain.Workspace, title string) (domain.Record, bool) {
	for _, record := range workspace.Records {
		if record.Title == title {
			return record, true
		}
	}
	return domain.Record{}, false
}

// longPrepBody is a realistic run sheet: a dozen typed beats with prose.
func longPrepBody() string {
	kinds := []string{"Scene", "Encounter", "Clue", "NPC", "Location", "Treasure"}
	var body strings.Builder
	for index := 1; index <= 12; index++ {
		fmt.Fprintf(&body, "## %s: Beat %d\n\nThe party presses on through part %d of the crypt. Read the room and adjust pacing.\n\n", kinds[(index-1)%len(kinds)], index, index)
	}
	return body.String()
}

// CoreJourneys are the realistic-pressure workflows from task #122.
func CoreJourneys() []Journey {
	return []Journey{
		{
			Name: "find-npc", Goal: "Find an NPC's table summary in three actions", Budget: 3,
			Steps: []JourneyStep{{Key: "/"}, {Type: "Merrow"}, {Key: "enter"}},
			Check: func(h *Harness) error {
				if !h.Frame().Contains("Father Merrow") {
					return fmt.Errorf("Father Merrow is not on screen")
				}
				merrow, _ := findRecordByTitle(h.Model.workspace, "Father Merrow")
				if !h.Frame().Contains(firstWords(merrow.Summary, 3)) {
					return fmt.Errorf("Merrow's summary is not visible")
				}
				return nil
			},
		},
		{
			Name: "capture-live", Goal: "Capture an unexpected fact without leaving live play", Budget: 3,
			Setup: func(h *Harness) error {
				h.Key("s")
				h.Type("The party enters the crypt")
				h.Key("enter")
				if h.Model.session == nil {
					return fmt.Errorf("live play did not start")
				}
				return nil
			},
			Steps: []JourneyStep{{Key: "ctrl+n"}, {Type: "Merrow lied about the silver key"}, {Key: "enter"}},
			Check: func(h *Harness) error {
				if h.Model.session == nil {
					return fmt.Errorf("capture left live play")
				}
				if len(h.Model.session.Entries) != 1 {
					return fmt.Errorf("capture wrote to the transcript: %d entries", len(h.Model.session.Entries))
				}
				captures := domain.UnfiledCaptures(h.Model.workspace, h.Model.workspace.Scope)
				if len(captures) != 1 || captures[0].Authority != domain.Draft {
					return fmt.Errorf("expected one draft capture, got %d", len(captures))
				}
				if h.Model.capture.Open || !h.Frame().Contains("SESSION TRANSCRIPT") {
					return fmt.Errorf("the transcript is not back on screen")
				}
				return nil
			},
		},
		{
			// Search's Enter opens the result in the navigator directly.
			Name: "recover-thread", Goal: "Recover an open thread's state and history", Budget: 3,
			Steps: []JourneyStep{{Key: "/"}, {Type: "Reliquary"}, {Key: "enter"}},
			Check: func(h *Harness) error {
				frame := h.Frame()
				if h.Model.preview != nil {
					return fmt.Errorf("still in preview")
				}
				for _, want := range []string{"Missing Reliquary", "· open", "WHY NOW"} {
					if !frame.Contains(want) {
						return fmt.Errorf("%q is not on screen", want)
					}
				}
				return nil
			},
		},
		{
			Name: "prep-by-beats", Goal: "Run a long prep document beat by beat during play", Budget: 8,
			Setup: func(h *Harness) error {
				plan := domain.PlannedNotes{
					ID: "plan-journey", Title: "Crypt Descent", Scope: h.Model.workspace.Scope,
					Body: longPrepBody(), CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
				}
				h.Model.workspace.PlannedNotes = append(h.Model.workspace.PlannedNotes, plan)
				h.Model.selectedPlanID = plan.ID
				h.Key("s")
				if h.Model.sessionPlannedNotes() == nil {
					return fmt.Errorf("live play did not load the prep")
				}
				return nil
			},
			Steps: []JourneyStep{{Key: "tab"}, {Key: "j"}, {Key: "j"}, {Key: "j"}, {Key: "d"}},
			Until: map[int]func(*Harness) bool{
				0: func(h *Harness) bool { return h.Model.sessionFocus() == prefs.PaneCampaign },
			},
			Check: func(h *Harness) error {
				frame := h.Frame()
				if !frame.Contains("4/12") {
					return fmt.Errorf("the run sheet does not show beat 4/12")
				}
				if !frame.Contains("Beat 4") {
					return fmt.Errorf("beat 4's title is not visible")
				}
				return nil
			},
		},
		{
			Name: "chase-and-return", Goal: "Follow a reference and come back to the same place", Budget: 6,
			Steps: []JourneyStep{{Key: "right"}, {Key: "right"}, {Key: "enter"}, {Key: "enter"}, {Key: "backspace"}},
			Check: func(h *Harness) error {
				if h.Model.selectedID != "npc-captain-vale" {
					return fmt.Errorf("back landed on %q, not Captain Vale", h.Model.selectedID)
				}
				if h.Model.layout.Focus != prefs.PaneDetail || h.Model.historyCursor != 0 {
					return fmt.Errorf("back lost the detail link position")
				}
				if !h.Frame().Contains("Captain Vale") {
					return fmt.Errorf("Captain Vale is not on screen")
				}
				return nil
			},
		},
		{
			Name: "clear-inbox", Goal: "Clear a mixed post-session inbox", Budget: 6, ApprovesCanon: true,
			Setup: func(h *Harness) error {
				h.Key("s")
				h.SetSessionDraft("@Captain Vale bars the watch house door")
				h.Key("enter")
				h.SetSessionDraft("Rain hammers the monastery roof")
				h.Key("enter")
				h.SetSessionDraft("A bell tolls three times at midnight")
				h.Key("enter")
				if h.Model.session == nil || len(h.Model.session.Entries) != 3 {
					return fmt.Errorf("setup could not capture three entries")
				}
				return nil
			},
			Steps: []JourneyStep{{Key: "ctrl+e"}, {Key: "a"}, {Key: "d"}, {Key: "x"}, {Key: "a"}},
			Check: func(h *Harness) error {
				if !h.Model.reconciling {
					return fmt.Errorf("the inbox closed before it was cleared")
				}
				if remaining := len(h.Model.unresolvedReconciliationItems()); remaining != 0 {
					return fmt.Errorf("%d items remain", remaining)
				}
				if !h.Frame().Contains("Inbox complete") {
					return fmt.Errorf("completion is not on screen")
				}
				return nil
			},
		},
	}
}

func firstWords(text string, count int) string {
	words := strings.Fields(text)
	if len(words) > count {
		words = words[:count]
	}
	return strings.Join(words, " ")
}

// RunCoreJourneys runs every core journey at every journey size.
func RunCoreJourneys() []JourneyResult {
	var results []JourneyResult
	for _, journey := range CoreJourneys() {
		for _, size := range JourneySizes {
			results = append(results, RunJourney(journey, size[0], size[1]))
		}
	}
	return results
}
