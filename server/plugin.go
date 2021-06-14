package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/larkox/mattermost-plugin-badges/badgesmodel"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	BotUserID string

	gameManager GameManager
	router      *mux.Router
	badgesMap   map[string]badgesmodel.BadgeID
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
func (p *Plugin) OnActivate() error {
	botID, err := p.Helpers.EnsureBot(&model.Bot{
		Username:    "chess",
		DisplayName: "Chess Bot",
		Description: "Created by the Chess plugin.",
	})
	if err != nil {
		return errors.Wrap(err, "failed to ensure todo bot")
	}
	p.BotUserID = botID

	p.gameManager = NewGameManager(p.API, botID, p.GrantBadge)

	p.initializeAPI()
	p.EnsureBadges()

	return p.API.RegisterCommand(getCommand())
}
