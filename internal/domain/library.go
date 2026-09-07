package domain

import (
	"fmt"
	"strings"
)

// EnsureLibrary fills Library from Scope and Records when the catalog is empty
// so older workspaces still present a world/campaign picker.
func (w *Workspace) EnsureLibrary() {
	if len(w.Library) > 0 {
		return
	}
	byWorld := map[string]*WorldRef{}
	order := make([]string, 0)
	add := func(scope Scope) {
		if scope.WorldID == "" {
			return
		}
		world, ok := byWorld[scope.WorldID]
		if !ok {
			world = &WorldRef{ID: scope.WorldID, Name: scope.WorldName}
			if world.Name == "" {
				world.Name = scope.WorldID
			}
			byWorld[scope.WorldID] = world
			order = append(order, scope.WorldID)
		}
		if scope.CampaignID == "" {
			return
		}
		for _, campaign := range world.Campaigns {
			if campaign.ID == scope.CampaignID {
				return
			}
		}
		name := scope.Campaign
		if name == "" {
			name = scope.CampaignID
		}
		world.Campaigns = append(world.Campaigns, CampaignRef{ID: scope.CampaignID, Name: name})
	}
	add(w.Scope)
	for _, record := range w.Records {
		add(record.Scope)
	}
	for _, session := range w.Sessions {
		add(session.Scope)
	}
	for _, id := range order {
		w.Library = append(w.Library, *byWorld[id])
	}
}

// FindWorld returns a library world by ID.
func (w Workspace) FindWorld(id string) (WorldRef, bool) {
	for _, world := range w.Library {
		if world.ID == id {
			return world, true
		}
	}
	return WorldRef{}, false
}

// ScopeFor builds an active Scope from library IDs.
func (w Workspace) ScopeFor(worldID, campaignID string) (Scope, bool) {
	world, ok := w.FindWorld(worldID)
	if !ok {
		return Scope{}, false
	}
	scope := Scope{WorldID: world.ID, WorldName: world.Name}
	for _, campaign := range world.Campaigns {
		if campaign.ID == campaignID {
			scope.CampaignID = campaign.ID
			scope.Campaign = campaign.Name
			return scope, true
		}
	}
	return Scope{}, false
}

// RenameWorld changes a world's display name. IDs stay put; every matching
// Scope.WorldName is rewritten.
func (w *Workspace) RenameWorld(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("world name is required")
	}
	found := false
	for index := range w.Library {
		if w.Library[index].ID != id {
			continue
		}
		w.Library[index].Name = name
		found = true
		break
	}
	if !found {
		return fmt.Errorf("world %q not found", id)
	}
	w.rewriteScopes(func(scope *Scope) {
		if scope.WorldID == id {
			scope.WorldName = name
		}
	})
	return nil
}

// RenameCampaign changes a campaign's display name. IDs stay put.
func (w *Workspace) RenameCampaign(worldID, campaignID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("campaign name is required")
	}
	found := false
	for i := range w.Library {
		if w.Library[i].ID != worldID {
			continue
		}
		for j := range w.Library[i].Campaigns {
			if w.Library[i].Campaigns[j].ID != campaignID {
				continue
			}
			w.Library[i].Campaigns[j].Name = name
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("campaign %q not found", campaignID)
	}
	w.rewriteScopes(func(scope *Scope) {
		if scope.WorldID == worldID && scope.CampaignID == campaignID {
			scope.Campaign = name
		}
	})
	return nil
}

// DeleteCampaign removes a campaign and every record, session, prep note, and
// collection scoped to it. World-shared rows stay. Sibling campaigns stay.
func (w *Workspace) DeleteCampaign(worldID, campaignID string) error {
	if worldID == "" || campaignID == "" {
		return fmt.Errorf("campaign is required")
	}
	removed := false
	for i := range w.Library {
		if w.Library[i].ID != worldID {
			continue
		}
		next := w.Library[i].Campaigns[:0]
		for _, campaign := range w.Library[i].Campaigns {
			if campaign.ID == campaignID {
				removed = true
				continue
			}
			next = append(next, campaign)
		}
		w.Library[i].Campaigns = next
	}
	if !removed {
		return fmt.Errorf("campaign %q not found", campaignID)
	}
	w.dropScoped(worldID, campaignID)
	return nil
}

// DeleteWorld removes a world and every campaign, wiki row, session, prep
// note, and collection inside it.
func (w *Workspace) DeleteWorld(id string) error {
	if id == "" {
		return fmt.Errorf("world is required")
	}
	next := w.Library[:0]
	found := false
	for _, world := range w.Library {
		if world.ID == id {
			found = true
			continue
		}
		next = append(next, world)
	}
	if !found {
		return fmt.Errorf("world %q not found", id)
	}
	w.Library = next
	w.dropScoped(id, "")
	return nil
}

func (w *Workspace) rewriteScopes(fn func(*Scope)) {
	if w == nil {
		return
	}
	fn(&w.Scope)
	for index := range w.Records {
		fn(&w.Records[index].Scope)
	}
	for index := range w.Sessions {
		fn(&w.Sessions[index].Scope)
	}
	for index := range w.PlannedNotes {
		fn(&w.PlannedNotes[index].Scope)
	}
	for index := range w.Collections {
		fn(&w.Collections[index].Scope)
	}
}

func (w *Workspace) dropScoped(worldID, campaignID string) {
	keep := func(scope Scope) bool {
		if scope.WorldID != worldID {
			return true
		}
		if campaignID == "" {
			return false
		}
		return scope.CampaignID != campaignID
	}
	droppedSessions := map[string]bool{}
	records := w.Records[:0]
	for _, record := range w.Records {
		if keep(record.Scope) {
			records = append(records, record)
		}
	}
	w.Records = records
	sessions := w.Sessions[:0]
	for _, session := range w.Sessions {
		if keep(session.Scope) {
			sessions = append(sessions, session)
			continue
		}
		droppedSessions[session.ID] = true
	}
	w.Sessions = sessions
	plans := w.PlannedNotes[:0]
	for _, plan := range w.PlannedNotes {
		if keep(plan.Scope) {
			plans = append(plans, plan)
		}
	}
	w.PlannedNotes = plans
	cols := w.Collections[:0]
	for _, col := range w.Collections {
		if keep(col.Scope) {
			cols = append(cols, col)
		}
	}
	w.Collections = cols
	recons := w.Reconciliations[:0]
	for _, recon := range w.Reconciliations {
		if droppedSessions[recon.SessionID] {
			continue
		}
		recons = append(recons, recon)
	}
	w.Reconciliations = recons
	w.clearMissingScope()
}

func (w *Workspace) clearMissingScope() {
	if w.Scope.WorldID == "" {
		return
	}
	world, ok := w.FindWorld(w.Scope.WorldID)
	if !ok {
		w.Scope = Scope{}
		return
	}
	w.Scope.WorldName = world.Name
	if w.Scope.CampaignID == "" {
		return
	}
	for _, campaign := range world.Campaigns {
		if campaign.ID == w.Scope.CampaignID {
			w.Scope.Campaign = campaign.Name
			return
		}
	}
	w.Scope.CampaignID = ""
	w.Scope.Campaign = ""
}
