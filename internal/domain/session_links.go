package domain

// Associate adds a wiki entity to the session cast, deduped by RecordID.
func (s SessionRecord) Associate(link EntityLink) SessionRecord {
	if link.RecordID == "" && link.Text == "" {
		return s
	}
	for _, existing := range s.Links {
		if link.RecordID != "" && existing.RecordID == link.RecordID {
			return s
		}
		if link.RecordID == "" && existing.Text == link.Text {
			return s
		}
	}
	s.Links = append(s.Links, link)
	return s
}

// Disassociate removes a wiki entity from the session cast by RecordID.
func (s SessionRecord) Disassociate(recordID string) SessionRecord {
	if recordID == "" {
		return s
	}
	out := make([]EntityLink, 0, len(s.Links))
	for _, link := range s.Links {
		if link.RecordID == recordID {
			continue
		}
		out = append(out, link)
	}
	s.Links = out
	return s
}

// SeedLinksFrom copies planned-note associations onto the session cast.
func (s SessionRecord) SeedLinksFrom(plan PlannedNotes) SessionRecord {
	for _, link := range plan.Links {
		s = s.Associate(link)
	}
	return s
}

// HasLink reports whether the session cast includes recordID.
func (s SessionRecord) HasLink(recordID string) bool {
	if recordID == "" {
		return false
	}
	for _, link := range s.Links {
		if link.RecordID == recordID {
			return true
		}
	}
	return false
}
