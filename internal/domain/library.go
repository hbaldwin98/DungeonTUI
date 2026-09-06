package domain

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
