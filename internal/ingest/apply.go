package ingest

import (
	"fmt"
	"strings"
	"time"

	"github.com/hbaldwin98/dungeon/internal/classify"
	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
)

// Options control where ingested content lands.
type Options struct {
	Kind     domain.SourceKind
	Scope    domain.Scope
	Path     string
	Fetcher  fivetools.Fetcher // tests; live import uses HTTP or DataDir
	DataDir  string            // optional local 5e.tools checkout (contains data/)
	Progress func(Progress)    // optional live status for the TUI or CLI
}

// Report is a short digest of what Apply wrote.
type Report struct {
	SourceID  string
	Kind      domain.SourceKind
	Title     string
	Records   int
	Planned   int
	Linked    int
	Reference bool
}

// Apply parses markdown and upserts a source document plus records into ws.
func Apply(ws domain.Workspace, markdown string, opts Options) (domain.Workspace, Report, error) {
	book := Parse(markdown, opts.Kind)
	if title := titleFromPath(opts.Path); genericSourceTitle(book.Title) && title != "" {
		book.Title = title
	}
	if book.Title == "" {
		return ws, Report{}, fmt.Errorf("markdown has no title heading")
	}
	doc := domain.SourceDocument{
		ID:      "src-" + slug(book.Title),
		Title:   book.Title,
		Edition: book.Edition,
		Kind:    book.Kind,
		Path:    opts.Path,
	}
	if err := doc.Validate(); err != nil {
		return ws, Report{}, err
	}

	opts.report(StageConvert, "Parsing "+doc.Title, 0, 0)
	ws.EnsureLibrary()
	ws.StripSourceContent(doc.ID)
	ws.UpsertSource(doc)

	scope := opts.Scope
	if scope.WorldID == "" {
		scope = ws.Scope
	}
	libraryScope := domain.Scope{}
	campaignScope := scope

	var records []domain.Record
	var plans []domain.PlannedNotes
	now := time.Now().UTC()

	switch book.Kind {
	case domain.SourceBestiary:
		records = bestiaryRecords(book, doc, libraryScope)
	case domain.SourceRules:
		records = rulesRecords(book, doc, libraryScope)
	default:
		records, plans = adventureRecords(book, doc, campaignScope, now)
	}

	for _, record := range records {
		if err := record.Validate(); err != nil {
			return ws, Report{}, err
		}
	}
	opts.report(StageClassify, fmt.Sprintf("Classifying %d records", len(records)), len(records), len(records))
	classify.OrganizeRecords(records)
	opts.report(StageWrite, "Writing wiki", len(records), len(records))
	ws.Records = append(ws.Records, records...)
	ws.PlannedNotes = append(ws.PlannedNotes, plans...)

	linked := 0
	if book.Kind == domain.SourceAdventure {
		enabled := append([]string(nil), ws.EnabledSourceIDs(campaignScope)...)
		enabled = append(enabled, doc.ID)
		linked = associateMentions(&ws, campaignScope, enabled)
	}

	if campaignScope.CampaignID != "" {
		ws.EnableSource(campaignScope.WorldID, campaignScope.CampaignID, doc.ID)
	}

	return ws, Report{
		SourceID: doc.ID,
		Kind:     book.Kind,
		Title:    doc.Title,
		Records:  len(records),
		Planned:  len(plans),
		Linked:   linked,
	}, nil
}

func bestiaryRecords(book ParsedBook, doc domain.SourceDocument, scope domain.Scope) []domain.Record {
	out := make([]domain.Record, 0)
	seen := map[string]bool{}
	for _, entry := range book.Entries {
		if entry.Level != 2 || isSkippedBestiary(entry.Title) {
			continue
		}
		id := doc.ID + "-creature-" + slug(entry.Title)
		if seen[id] {
			continue
		}
		seen[id] = true
		body := truncate(entry.Body, 4000)
		if strings.TrimSpace(body) == "" {
			body = "Stat block was not present in the markdown export. Name and aliases are from " + doc.Title + ".\n"
		}
		aliases := uniqueStrings(append([]string{singular(entry.Title)}, book.Aliases[entry.Title]...))
		out = append(out, stampSourceFolder(doc, domain.Record{
			ID:        id,
			Type:      domain.Creature,
			Title:     entry.Title,
			Summary:   firstNonEmpty(summaryOf(body), "Creature from "+doc.Title+"."),
			Body:      body,
			Authority: domain.Canon,
			Scope:     scope,
			Source:    doc.Title,
			SourceID:  doc.ID,
			Aliases:   aliases,
			Tags:      []string{"source", slug(string(doc.Kind))},
		}))
	}
	return out
}

func rulesRecords(book ParsedBook, doc domain.SourceDocument, scope domain.Scope) []domain.Record {
	out := make([]domain.Record, 0)
	seen := map[string]bool{}
	for _, entry := range book.Entries {
		if entry.Level != 2 {
			continue
		}
		lower := strings.ToLower(entry.Title)
		if lower == "credits" || lower == "what you need" || lower == "using this book" {
			continue
		}
		id := doc.ID + "-rule-" + slug(entry.Title)
		if seen[id] {
			continue
		}
		seen[id] = true
		body := truncate(entry.Body, 4000)
		out = append(out, stampSourceFolder(doc, domain.Record{
			ID:        id,
			Type:      domain.Rule,
			Title:     entry.Title,
			Summary:   firstNonEmpty(summaryOf(body), "Rule from "+doc.Title+"."),
			Body:      body,
			Authority: domain.Canon,
			Scope:     scope,
			Source:    doc.Title,
			SourceID:  doc.ID,
			Tags:      []string{"source", "rules"},
		}))
	}
	return out
}

func adventureRecords(book ParsedBook, doc domain.SourceDocument, scope domain.Scope, now time.Time) ([]domain.Record, []domain.PlannedNotes) {
	records := make([]domain.Record, 0)
	plans := make([]domain.PlannedNotes, 0)
	seen := map[string]bool{}
	add := func(rec domain.Record) {
		if rec.ID == "" || seen[rec.ID] {
			return
		}
		seen[rec.ID] = true
		records = append(records, rec)
	}
	later := markdownChapterTitles(book.Entries)

	for _, entry := range book.Entries {
		titleLower := strings.ToLower(entry.Title)
		if classify.NPCRosterTitle(entry.Title) {
			for _, row := range parseNPCTable(entry.Body) {
				site := firstNumberedParent(entry.Path)
				add(npcRecord(doc, scope, row[0], row[1], site))
			}
			continue
		}
		for _, name := range parseItemList(entry.Body) {
			add(itemRecord(doc, scope, name, "Magic item from "+doc.Title+"."))
		}
		if underFrontMatterPath(entry.Path) {
			if later[titleLower] {
				continue
			}
			add(noteRecord(doc, scope, entry, strings.Join(entry.Path[:len(entry.Path)-1], "/")))
			continue
		}
		group := locationGroup(entry.Path)
		tags := tagsForPath(entry.Path)
		if entry.Level >= 2 && !classify.FrontMatter(entry.Title) {
			tags = append(tags, "5e-section")
		}
		if classify.NumberedRoom(entry.Title) {
			tags = append(tags, "room")
		}
		switch classify.Assign(entry.Title, group, tags) {
		case domain.Location:
			add(locationRecord(doc, scope, entry.Title, entry.Body, group, tags))
			if entry.Level == 1 {
				plans = append(plans, plannedFromPart(doc, scope, entry, now))
			}
		default:
			add(noteRecord(doc, scope, entry, group))
		}
	}
	domain.NestOverviewFolders(records)
	return records, plans
}

func markdownChapterTitles(entries []ParsedEntry) map[string]bool {
	out := map[string]bool{}
	for _, entry := range entries {
		if underFrontMatterPath(entry.Path) {
			continue
		}
		if classify.FrontMatter(entry.Title) && entry.Level == 1 {
			continue
		}
		out[strings.ToLower(entry.Title)] = true
	}
	return out
}

func underFrontMatterPath(path []string) bool {
	if len(path) < 2 {
		return false
	}
	return classify.FrontMatter(path[0])
}

func locationGroup(path []string) string {
	if len(path) < 2 {
		return ""
	}
	parts := make([]string, 0, len(path)-1)
	for _, part := range path[:len(path)-1] {
		if classify.FrontMatter(part) {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

func stampSourceFolder(doc domain.SourceDocument, rec domain.Record) domain.Record {
	rec.Folder = domain.SourceFolderWithGroup(doc.Title, rec, "")
	return rec
}

func locationRecord(doc domain.SourceDocument, scope domain.Scope, title, body, group string, tags []string) domain.Record {
	body = truncate(body, 6000)
	idKey := title
	if group != "" && !strings.EqualFold(group, title) {
		idKey = group + " " + title
	}
	return domain.Record{
		ID:        doc.ID + "-location-" + slug(idKey),
		Type:      domain.Location,
		Title:     title,
		Summary:   firstNonEmpty(summaryOf(body), "Location from "+doc.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Scope:     scope,
		Source:    doc.Title,
		SourceID:  doc.ID,
		Tags:      uniqueStrings(append([]string{"source"}, tags...)),
		Folder:    domain.SourceFolderWithGroup(doc.Title, domain.Record{Type: domain.Location, Tags: tags, Source: doc.Title, SourceID: doc.ID}, group),
	}
}

func noteRecord(doc domain.SourceDocument, scope domain.Scope, entry ParsedEntry, group string) domain.Record {
	body := truncate(entry.Body, 6000)
	return domain.Record{
		ID:        doc.ID + "-note-" + slug(entry.Title),
		Type:      domain.Note,
		Title:     entry.Title,
		Summary:   firstNonEmpty(summaryOf(body), "Notes from "+doc.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Scope:     scope,
		Source:    doc.Title,
		SourceID:  doc.ID,
		Tags:      []string{"source", "adventure"},
		Folder:    domain.SourceFolderWithGroup(doc.Title, domain.Record{Type: domain.Note, Tags: []string{"adventure"}, Source: doc.Title, SourceID: doc.ID}, group),
	}
}

func npcRecord(doc domain.SourceDocument, scope domain.Scope, name, summary, group string) domain.Record {
	clean := strings.Trim(name, "* ")
	body := strings.TrimSpace(summary)
	if body == "" {
		body = "NPC from " + doc.Title + "."
	}
	return domain.Record{
		ID:        doc.ID + "-npc-" + slug(clean),
		Type:      domain.NPC,
		Title:     clean,
		Summary:   firstNonEmpty(summary, "NPC from "+doc.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Scope:     scope,
		Source:    doc.Title,
		SourceID:  doc.ID,
		Tags:      []string{"source", "adventure"},
		Folder:    domain.SourceFolderWithGroup(doc.Title, domain.Record{Type: domain.NPC, Tags: []string{"adventure"}, Source: doc.Title, SourceID: doc.ID}, group),
	}
}

func creatureRecord(doc domain.SourceDocument, scope domain.Scope, entry ParsedEntry, extra []string) domain.Record {
	body := truncate(entry.Body, 3000)
	if strings.TrimSpace(stripHTML(body)) == "" || len(strings.TrimSpace(body)) < 40 {
		body = "Adventure-specific creature from " + doc.Title + ". Stat block was not present in the markdown export.\n"
	}
	return stampSourceFolder(doc, domain.Record{
		ID:        doc.ID + "-creature-" + slug(entry.Title),
		Type:      domain.Creature,
		Title:     entry.Title,
		Summary:   firstNonEmpty(summaryOf(entry.Body), "Creature from "+doc.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Scope:     scope,
		Source:    doc.Title,
		SourceID:  doc.ID,
		Aliases:   uniqueStrings(append([]string{singular(entry.Title)}, extra...)),
		Tags:      []string{"source", "adventure"},
	})
}

func itemRecord(doc domain.SourceDocument, scope domain.Scope, name, summary string) domain.Record {
	return stampSourceFolder(doc, domain.Record{
		ID:        doc.ID + "-item-" + slug(name),
		Type:      domain.Item,
		Title:     name,
		Summary:   firstNonEmpty(summary, "Item from "+doc.Title+"."),
		Body:      firstNonEmpty(summary, "Item from "+doc.Title+"."),
		Authority: domain.Canon,
		Scope:     scope,
		Source:    doc.Title,
		SourceID:  doc.ID,
		Tags:      []string{"source", "adventure"},
	})
}

func plannedFromPart(doc domain.SourceDocument, scope domain.Scope, entry ParsedEntry, now time.Time) domain.PlannedNotes {
	body := truncate(entry.Body, 2500)
	if loc := strings.TrimSpace(entry.Title); loc != "" {
		body = "#location " + loc + "\n\n" + body
	}
	plan := domain.PlannedNotes{
		ID:        doc.ID + "-prep-" + slug(entry.Title),
		Title:     entry.Title,
		Scope:     scope,
		Body:      body,
		SourceID:  doc.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return plan
}

func tagsForPath(path []string) []string {
	tags := []string{"adventure"}
	if len(path) > 0 {
		tags = append(tags, slug(path[0]))
	}
	if len(path) > 1 {
		tags = append(tags, slug(path[1]))
	}
	return tags
}

func associateMentions(ws *domain.Workspace, scope domain.Scope, enabled []string) int {
	linked := 0
	visible := make([]domain.Record, 0)
	for _, record := range ws.Records {
		if domain.RecordVisibleIn(record, scope, enabled) {
			visible = append(visible, record)
		}
	}
	for i, record := range ws.Records {
		if record.SourceID == "" || record.Scope.CampaignID != scope.CampaignID {
			continue
		}
		if record.Type != domain.Location && record.Type != domain.Note {
			continue
		}
		refs := extractBoldNames(record.Body)
		if len(refs) == 0 {
			continue
		}
		var extra []string
		seen := map[string]bool{}
		for _, name := range refs {
			target, ok := matchEntity(visible, name)
			if !ok || seen[target.ID] {
				continue
			}
			seen[target.ID] = true
			extra = append(extra, "@"+target.Title)
			linked++
		}
		if len(extra) == 0 {
			continue
		}
		block := "\n\nReferenced: " + strings.Join(extra, ", ") + "\n"
		if strings.Contains(record.Body, "Referenced: ") {
			continue
		}
		ws.Records[i].Body = strings.TrimRight(record.Body, "\n") + block
	}
	for i, plan := range ws.PlannedNotes {
		if plan.SourceID == "" || plan.Scope.CampaignID != scope.CampaignID {
			continue
		}
		ws.PlannedNotes[i] = plan.RefreshPlannedLinks(visible)
	}
	return linked
}

func extractBoldNames(body string) []string {
	matches := boldName.FindAllStringSubmatch(body, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		if len(name) < 3 || strings.Contains(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return uniqueStrings(names)
}

func matchEntity(records []domain.Record, name string) (domain.Record, bool) {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return domain.Record{}, false
	}
	singularNeedle := singular(needle)

	score := func(record domain.Record) int {
		title := strings.ToLower(record.Title)
		best := 0
		switch {
		case title == needle:
			best = 6
		case title == singularNeedle || singular(title) == needle:
			best = 5
		case strings.HasPrefix(title, needle) || strings.HasPrefix(title, singularNeedle):
			best = 3
		}
		for _, alias := range record.Aliases {
			al := strings.ToLower(alias)
			switch {
			case al == needle || al == singularNeedle:
				if best < 6 {
					best = 5
				}
			case strings.HasPrefix(al, needle) || strings.HasPrefix(al, singularNeedle):
				if best < 3 {
					best = 3
				}
			}
		}
		if record.Scope.CampaignID != "" && best > 0 {
			best++
		}
		return best
	}

	bestScore := 0
	var best domain.Record
	for _, record := range records {
		if record.Type != domain.Creature && record.Type != domain.NPC && record.Type != domain.Item && record.Type != domain.Faction {
			continue
		}
		s := score(record)
		if s > bestScore {
			best = record
			bestScore = s
		}
	}
	if bestScore == 0 {
		return domain.Record{}, false
	}
	return best, true
}

func singular(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "ies") && len(s) > 3 {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(lower, "sses") || strings.HasSuffix(lower, "shes") || strings.HasSuffix(lower, "ches") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") && len(s) > 3 {
		return s[:len(s)-1]
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
