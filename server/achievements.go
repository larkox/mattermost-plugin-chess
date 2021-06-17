package main

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/larkox/mattermost-plugin-badges/badgesmodel"
)

func (p *Plugin) EnsureBadges() {
	badges := []*badgesmodel.Badge{
		{
			Name:        AchievementNameWinner,
			Description: "Win a game of chess",
			Image:       "crown",
			ImageType:   badgesmodel.ImageTypeEmoji,
			Multiple:    false,
		},
	}

	reqBody := badgesmodel.EnsureBadgesRequest{
		Badges: badges,
		BotID:  p.BotUserID,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		p.API.LogDebug("Cannot marshal ensure request", "err", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, badgesmodel.PluginPath+badgesmodel.PluginAPIPath+badgesmodel.PluginAPIPathEnsure, bytes.NewReader(b))
	if err != nil {
		p.API.LogDebug("Cannot create http request", "err", err)
		return
	}

	resp := p.API.PluginHTTP(req)
	if resp.StatusCode != http.StatusOK {
		p.API.LogDebug("Plugin request failed", "req", req, "resp", resp)
		return
	}

	var newBadges []badgesmodel.Badge
	err = json.NewDecoder(resp.Body).Decode(&newBadges)
	if err != nil {
		p.API.LogDebug("Cannot unmarshal response", "err", err)
		return
	}

	p.badgesMap = map[string]badgesmodel.BadgeID{}
	for _, badge := range newBadges {
		p.badgesMap[badge.Name] = badge.ID
	}
}

func (p *Plugin) GrantBadge(name string, userID string) {
	if p.badgesMap == nil {
		p.API.LogDebug("No badges map")
		return
	}

	badgeID, ok := p.badgesMap[name]
	if !ok {
		p.API.LogDebug("Achievement not recognized")
		return
	}

	grantReq := badgesmodel.GrantBadgeRequest{
		BadgeID: badgeID,
		UserID:  userID,
		BotID:   p.BotUserID,
	}

	b, err := json.Marshal(grantReq)
	if err != nil {
		p.API.LogDebug("Cannot marshal grant request")
		return
	}

	req, err := http.NewRequest(http.MethodPost, badgesmodel.PluginPath+badgesmodel.PluginAPIPath+badgesmodel.PluginAPIPathGrant, bytes.NewReader(b))
	if err != nil {
		p.API.LogDebug("Cannot create request")
	}

	resp := p.API.PluginHTTP(req)
	if resp.StatusCode != http.StatusOK {
		p.API.LogDebug("Plugin request failed", "req", req, "resp", resp)
		return
	}

	p.API.LogDebug("Achievement granted", "badgeID", badgeID, "userID", userID, "botID", p.BotUserID)
}
