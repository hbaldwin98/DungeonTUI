package search

import (
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

// Document is one FTS row. JSON workspace stays the inspectable source of
// truth; these rows are a rebuildable index.
type Document struct {
	Kind         Kind
	ID           string
	SessionID    string
	Title        string
	Body         string
	Tags         []string
	Aliases      []string
	TypeLabel    string
	Authority    domain.Authority
	WorldID      string
	CampaignID   string
	SourceID     string
	IncludeEmpty bool
	Snippet      string
	Record       domain.Record
}

func DocumentsFromWorkspace(workspace domain.Workspace) []Document {
	docs := make([]Document, 0, len(workspace.Records)+len(workspace.PlannedNotes)+len(workspace.Sessions))
	for _, record := range workspace.Records {
		docs = append(docs, documentFromRecord(record))
	}
	for _, plan := range workspace.PlannedNotes {
		docs = append(docs, documentFromPrep(plan))
	}
	sessions := make(map[string]domain.SessionRecord, len(workspace.Sessions))
	for _, session := range workspace.Sessions {
		sessions[session.ID] = session
		docs = append(docs, documentFromSession(session)...)
	}
	for _, recon := range workspace.Reconciliations {
		session, ok := sessions[recon.SessionID]
		if !ok {
			continue
		}
		docs = append(docs, documentFromRecon(recon, session))
	}
	return docs
}

func DocumentFromReference(record domain.Record) Document {
	doc := documentFromRecord(record)
	doc.Kind = KindReference
	doc.IncludeEmpty = false
	doc.TypeLabel = "reference"
	return doc
}

func documentFromRecord(record domain.Record) Document {
	return Document{
		Kind:         KindRecord,
		ID:           record.ID,
		Title:        record.Title,
		Body:         strings.Join([]string{record.Summary, record.Body, record.Source, strings.Join(record.Tags, " ")}, " "),
		Tags:         record.Tags,
		Aliases:      record.Aliases,
		TypeLabel:    string(record.Type),
		Authority:    record.Authority,
		WorldID:      record.Scope.WorldID,
		CampaignID:   record.Scope.CampaignID,
		SourceID:     record.SourceID,
		IncludeEmpty: true,
		Record:       record,
	}
}

func documentFromPrep(plan domain.PlannedNotes) Document {
	return Document{
		Kind:       KindPrep,
		ID:         plan.ID,
		Title:      plan.Title,
		Body:       plan.Body,
		TypeLabel:  "prep",
		Authority:  domain.Draft,
		WorldID:    plan.Scope.WorldID,
		CampaignID: plan.Scope.CampaignID,
	}
}

func documentFromSession(session domain.SessionRecord) []Document {
	docs := []Document{{
		Kind:       KindSession,
		ID:         session.ID,
		Title:      session.Title,
		Body:       session.LocationName,
		TypeLabel:  "session",
		Authority:  domain.Canon,
		WorldID:    session.Scope.WorldID,
		CampaignID: session.Scope.CampaignID,
		Snippet:    session.LocationName,
	}}
	for _, entry := range session.Entries {
		if entry.Undone || strings.TrimSpace(entry.Text) == "" {
			continue
		}
		docs = append(docs, Document{
			Kind:       KindTranscript,
			ID:         entry.ID,
			SessionID:  session.ID,
			Title:      session.Title + "  “" + truncate(entry.Text, 40) + "”",
			Body:       entry.Text,
			TypeLabel:  "note",
			Authority:  domain.Canon,
			WorldID:    session.Scope.WorldID,
			CampaignID: session.Scope.CampaignID,
		})
	}
	return docs
}

func documentFromRecon(recon domain.ReconciliationRecord, session domain.SessionRecord) Document {
	body := recon.Title
	for _, item := range recon.Items {
		body += " " + item.Summary
	}
	return Document{
		Kind:       KindRecon,
		ID:         recon.ID,
		SessionID:  recon.SessionID,
		Title:      recon.Title,
		Body:       body,
		TypeLabel:  "recon",
		Authority:  domain.Draft,
		WorldID:    session.Scope.WorldID,
		CampaignID: session.Scope.CampaignID,
	}
}

func (d Document) result(score int, query string) Result {
	snippet := d.Snippet
	if snippet == "" {
		snippet = snippetAround(d.Body, query)
	}
	return Result{
		Kind:      d.Kind,
		ID:        d.ID,
		SessionID: d.SessionID,
		Title:     d.Title,
		Snippet:   snippet,
		Record:    d.Record,
		Score:     score,
	}
}

func (d Document) key() string {
	return string(d.Kind) + "\x00" + d.ID
}

func (d Document) scopeRecord() domain.Record {
	if d.Record.ID != "" {
		return d.Record
	}
	return domain.Record{
		Scope:    domain.Scope{WorldID: d.WorldID, CampaignID: d.CampaignID},
		SourceID: d.SourceID,
	}
}

func (d Document) scoreBody() string {
	if len(d.Tags) == 0 {
		return d.Body
	}
	return d.Body + " " + strings.Join(d.Tags, " ")
}
