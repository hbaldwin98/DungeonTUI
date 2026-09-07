package classify

import "github.com/hbaldwin98/dungeon/internal/domain"

// OrganizeRecords retypes outline records using Assign. NPC/item/creature/rule
// rows are left alone. Folder paths are not rewritten here; callers stamp
// folders after type is known.
func OrganizeRecords(records []domain.Record) {
	for i, rec := range records {
		switch rec.Type {
		case domain.NPC, domain.Item, domain.Creature, domain.Rule, domain.Character:
			continue
		}
		group := groupFromFolder(rec.Folder)
		want := Assign(rec.Title, group, rec.Tags)
		if want == rec.Type {
			continue
		}
		rec.Type = want
		if rec.Source != "" {
			rec.Folder = domain.SourceFolderWithGroup(rec.Source, rec, group)
		}
		records[i] = rec
	}
}
