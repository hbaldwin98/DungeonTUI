package domain

import "testing"

func TestAIContentCannotMasqueradeAsCanon(t *testing.T) {
	record := Record{
		ID:          "idea-1",
		Type:        Thread,
		Title:       "A generated idea",
		Authority:   Canon,
		IsAIContent: true,
	}

	if err := record.Validate(); err == nil {
		t.Fatal("expected an AI-authored canonical record to be rejected")
	}
}

func TestAuthorityMarkersAreDistinct(t *testing.T) {
	authorities := []Authority{Canon, Secret, Draft, Proposal, Unknown, Superseded}
	seen := make(map[string]bool)

	for _, authority := range authorities {
		marker := authority.Marker()
		if seen[marker] {
			t.Fatalf("marker %q is shared by multiple authorities", marker)
		}
		seen[marker] = true
	}
}
