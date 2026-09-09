package tui

import (
	"encoding/json"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/ingest/fivetools"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

func (m *Model) attachReferences() {
	m.adventures = nil
	enabled := map[string]bool{}
	for _, id := range m.workspace.EnabledSourceIDs(m.workspace.Scope) {
		enabled[id] = true
	}
	stored := map[string]fivetools.AdventureBook{}
	if store := sqliteStore(m.store); store != nil {
		if blobs, err := store.LoadAdventures(); err == nil {
			for _, book := range decodeAdventureBlobs(blobs) {
				stored[book.ID] = book
			}
		}
	}
	fetcher := m.adventureFetcher()
	for _, doc := range m.workspace.Sources {
		if doc.Kind != domain.SourceAdventure || !strings.HasPrefix(doc.ID, "src-5e-") || !enabled[doc.ID] {
			continue
		}
		if book, ok := stored[doc.ID]; ok {
			book.ID = doc.ID
			if book.Title == "" {
				book.Title = doc.Title
			}
			m.adventures = append(m.adventures, book)
			continue
		}
		if fetcher == nil {
			continue
		}
		book, err := fivetools.LoadAdventure(fetcher, fivetools.SourceCode(doc.ID), doc.Title)
		if err != nil {
			continue
		}
		book.ID = doc.ID
		book.Title = doc.Title
		m.adventures = append(m.adventures, book)
	}
}

func decodeAdventureBlobs(blobs []storage.Blob) []fivetools.AdventureBook {
	out := make([]fivetools.AdventureBook, 0, len(blobs))
	for _, blob := range blobs {
		var book fivetools.AdventureBook
		if err := json.Unmarshal(blob.Data, &book); err != nil {
			continue
		}
		if book.ID == "" {
			book.ID = blob.ID
		}
		out = append(out, book)
	}
	return out
}

func (m Model) adventureFetcher() fivetools.Fetcher {
	if m.toolsFetcher != nil {
		return m.toolsFetcher
	}
	return fivetools.CacheOnly(fivetools.DefaultCacheDir())
}

func (m Model) scopedReferenceSources() []domain.SourceDocument {
	enabled := map[string]bool{}
	for _, id := range m.workspace.EnabledSourceIDs(m.workspace.Scope) {
		enabled[id] = true
	}
	out := make([]domain.SourceDocument, 0)
	for _, doc := range m.workspace.Sources {
		if doc.Kind != domain.SourceAdventure || !strings.HasPrefix(doc.ID, "src-5e-") {
			continue
		}
		if !enabled[doc.ID] {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func (m Model) lookupReference(name string) (domain.Record, bool) {
	for _, book := range m.adventures {
		hit, ok := book.Lookup(name)
		if !ok {
			continue
		}
		return book.Record(hit, book.ID), true
	}
	return domain.Record{}, false
}

func (m Model) lookupReferenceID(id string) (domain.Record, bool) {
	if !fivetools.IsReferenceID(id) {
		return domain.Record{}, false
	}
	for _, book := range m.adventures {
		for _, hit := range book.Hits {
			rec := book.Record(hit, book.ID)
			if rec.ID == id {
				return rec, true
			}
		}
	}
	return domain.Record{}, false
}

func (m Model) lookupAny(id string) (domain.Record, bool) {
	if rec, ok := recordByID(m.workspace.Records, id); ok {
		return rec, true
	}
	return m.lookupReferenceID(id)
}

func (m Model) bindReferenceMentions(mentions []domain.Mention) []domain.Mention {
	if len(mentions) == 0 {
		return mentions
	}
	out := append([]domain.Mention(nil), mentions...)
	for i, mention := range out {
		if mention.RecordID != "" {
			continue
		}
		if rec, ok := m.lookupReference(mention.Text); ok {
			out[i].RecordID = rec.ID
		}
	}
	return out
}

func (m Model) resolveMentions(text string) []domain.Mention {
	return m.bindReferenceMentions(domain.MentionsIn(text, m.workspace.Records))
}

func (m Model) referenceSuggestions(query string, limit int) []Suggestion {
	if limit < 1 {
		return nil
	}
	out := make([]Suggestion, 0, limit)
	for _, book := range m.adventures {
		for _, hit := range book.Search(query, limit-len(out)) {
			rec := book.Record(hit, book.ID)
			label := "ref  " + rec.Title
			if rec.Source != "" {
				label += "  (" + rec.Source + ")"
			}
			out = append(out, Suggestion{
				Label:  label,
				Insert: "@" + rec.Title + " ",
				Record: &rec,
			})
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func (m Model) referenceDocuments() []searchsvc.Document {
	docs := make([]searchsvc.Document, 0)
	for _, book := range m.adventures {
		for _, hit := range book.Hits {
			docs = append(docs, searchsvc.DocumentFromReference(book.Record(hit, book.ID)))
		}
	}
	return docs
}

func (m Model) adventureBookBySource(id string) (fivetools.AdventureBook, bool) {
	for _, book := range m.adventures {
		if book.ID == id {
			return book, true
		}
	}
	return fivetools.AdventureBook{}, false
}
