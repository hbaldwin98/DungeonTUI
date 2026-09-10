package app

import (
	"fmt"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

// Service is the UI-independent transaction boundary for workspace writes.
type Service struct {
	Store storage.Store
}

func New(store storage.Store) Service {
	return Service{Store: store}
}

func (s Service) Save(ws domain.Workspace) error {
	if s.Store == nil {
		return nil
	}
	return s.Store.Save(ws)
}

// Commit clones current, applies mutate, and persists. Any error returns current unchanged.
func (s Service) Commit(current domain.Workspace, mutate func(domain.Workspace) (domain.Workspace, error)) (domain.Workspace, error) {
	next, err := mutate(current.Clone())
	if err != nil {
		return current, err
	}
	if err := s.Save(next); err != nil {
		return current, fmt.Errorf("persistence failed: %w", err)
	}
	return next, nil
}

func UpsertSession(ws domain.Workspace, session domain.SessionRecord) domain.Workspace {
	for index := range ws.Sessions {
		if ws.Sessions[index].ID == session.ID {
			ws.Sessions[index] = session
			return ws
		}
	}
	ws.Sessions = append(ws.Sessions, session)
	return ws
}

func UpsertRecon(ws domain.Workspace, recon domain.ReconciliationRecord) domain.Workspace {
	for index := range ws.Reconciliations {
		if ws.Reconciliations[index].SessionID == recon.SessionID {
			ws.Reconciliations[index] = recon
			return ws
		}
	}
	ws.Reconciliations = append(ws.Reconciliations, recon)
	return ws
}

func UpsertRecord(ws domain.Workspace, record domain.Record) (domain.Workspace, error) {
	if err := record.Validate(); err != nil {
		return ws, err
	}
	for index := range ws.Records {
		if ws.Records[index].ID == record.ID {
			ws.Records[index] = record
			return ws, nil
		}
	}
	ws.Records = append(ws.Records, record)
	return ws, nil
}

func DeleteRecord(ws domain.Workspace, id string) (domain.Workspace, error) {
	if id == "" {
		return ws, fmt.Errorf("record id is required")
	}
	out := make([]domain.Record, 0, len(ws.Records))
	found := false
	for _, record := range ws.Records {
		if record.ID == id {
			found = true
			continue
		}
		out = append(out, record)
	}
	if !found {
		return ws, fmt.Errorf("record %q not found", id)
	}
	ws.Records = out
	return ws, nil
}

func SupersedeRecord(ws domain.Workspace, id string) (domain.Workspace, error) {
	for index := range ws.Records {
		if ws.Records[index].ID != id {
			continue
		}
		if ws.Records[index].Authority == domain.Superseded {
			return ws, fmt.Errorf("record %q is already superseded", id)
		}
		ws.Records[index].Authority = domain.Superseded
		return ws, nil
	}
	return ws, fmt.Errorf("record %q not found", id)
}

func DeleteSession(ws domain.Workspace, id string) (domain.Workspace, error) {
	if id == "" {
		return ws, fmt.Errorf("session id is required")
	}
	sessions := make([]domain.SessionRecord, 0, len(ws.Sessions))
	found := false
	for _, session := range ws.Sessions {
		if session.ID == id {
			found = true
			continue
		}
		sessions = append(sessions, session)
	}
	if !found {
		return ws, fmt.Errorf("session %q not found", id)
	}
	ws.Sessions = sessions
	recons := make([]domain.ReconciliationRecord, 0, len(ws.Reconciliations))
	for _, recon := range ws.Reconciliations {
		if recon.SessionID == id {
			continue
		}
		recons = append(recons, recon)
	}
	ws.Reconciliations = recons
	return ws, nil
}

func EndSession(ws domain.Workspace, session domain.SessionRecord) (domain.Workspace, domain.ReconciliationRecord, error) {
	if session.ID == "" {
		return ws, domain.ReconciliationRecord{}, fmt.Errorf("no live session")
	}
	ws = UpsertSession(ws, session)
	for _, existing := range ws.Reconciliations {
		if existing.SessionID == session.ID {
			return ws, existing, nil
		}
	}
	recon := domain.BuildSessionReconciliation(session, ws.Records)
	ws = UpsertRecon(ws, recon)
	return ws, recon, nil
}

func ApplyRecon(ws domain.Workspace, reconIndex, itemIndex int) (domain.Workspace, error) {
	if reconIndex < 0 || reconIndex >= len(ws.Reconciliations) {
		return ws, fmt.Errorf("reconciliation not found")
	}
	recon := ws.Reconciliations[reconIndex]
	if itemIndex < 0 || itemIndex >= len(recon.Items) {
		return ws, fmt.Errorf("reconciliation item not found")
	}
	session := sessionByID(ws, recon.SessionID)
	updated, records, err := domain.ApplyReconItem(recon.Items[itemIndex], ws.Records, session)
	if err != nil {
		return ws, err
	}
	recon.Items[itemIndex] = updated
	ws.Reconciliations[reconIndex] = recon
	ws.Records = records
	return ws, nil
}

func RejectRecon(ws domain.Workspace, reconIndex, itemIndex int) (domain.Workspace, error) {
	if reconIndex < 0 || reconIndex >= len(ws.Reconciliations) {
		return ws, fmt.Errorf("reconciliation not found")
	}
	recon := ws.Reconciliations[reconIndex]
	if itemIndex < 0 || itemIndex >= len(recon.Items) {
		return ws, fmt.Errorf("reconciliation item not found")
	}
	if recon.Items[itemIndex].Status == domain.ReconRejected {
		return ws, nil
	}
	if !domain.ReconciliationUnresolved(recon.Items[itemIndex].Status) {
		return ws, fmt.Errorf("item is %s", recon.Items[itemIndex].Status)
	}
	recon.Items[itemIndex].Status = domain.ReconRejected
	ws.Reconciliations[reconIndex] = recon
	return ws, nil
}

func DeferRecon(ws domain.Workspace, reconIndex, itemIndex int) (domain.Workspace, error) {
	if reconIndex < 0 || reconIndex >= len(ws.Reconciliations) {
		return ws, fmt.Errorf("reconciliation not found")
	}
	recon := ws.Reconciliations[reconIndex]
	if itemIndex < 0 || itemIndex >= len(recon.Items) {
		return ws, fmt.Errorf("reconciliation item not found")
	}
	if recon.Items[itemIndex].Status == domain.ReconDeferred {
		return ws, nil
	}
	if recon.Items[itemIndex].Status != domain.ReconPending {
		return ws, fmt.Errorf("item is %s", recon.Items[itemIndex].Status)
	}
	recon.Items[itemIndex].Status = domain.ReconDeferred
	ws.Reconciliations[reconIndex] = recon
	return ws, nil
}

func EditReconMutation(ws domain.Workspace, reconIndex, itemIndex int, text string) (domain.Workspace, error) {
	if reconIndex < 0 || reconIndex >= len(ws.Reconciliations) {
		return ws, fmt.Errorf("reconciliation not found")
	}
	recon := ws.Reconciliations[reconIndex]
	if itemIndex < 0 || itemIndex >= len(recon.Items) {
		return ws, fmt.Errorf("reconciliation item not found")
	}
	if !domain.ReconciliationUnresolved(recon.Items[itemIndex].Status) {
		return ws, fmt.Errorf("only unresolved mutations can be edited")
	}
	if text == "" {
		return ws, fmt.Errorf("mutation text is required")
	}
	recon.Items[itemIndex].Mutation.Text = text
	ws.Reconciliations[reconIndex] = recon
	return ws, nil
}

func sessionByID(ws domain.Workspace, id string) domain.SessionRecord {
	for _, session := range ws.Sessions {
		if session.ID == id {
			return session
		}
	}
	return domain.SessionRecord{}
}
