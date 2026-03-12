package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/mattermost/mattermost-plugin-meet/server/command"
	"github.com/mattermost/mattermost-plugin-meet/server/store/kvstore"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	kvstore kvstore.KVStore
	client  *pluginapi.Client

	commandClient command.Command

	router *mux.Router
	botID  string

	configurationLock sync.RWMutex
	configuration     *configuration
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	botID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    "google-meet",
		DisplayName: "Google Meet",
		Description: "Created by the Google Meet plugin.",
	}, pluginapi.ProfileImagePath("assets/profile.png"))
	if err != nil {
		return fmt.Errorf("failed to ensure bot: %w", err)
	}
	p.botID = botID

	config := p.getConfiguration()
	encryptionKey := config.EncryptionKey
	if encryptionKey == "" {
		encryptionKey = "default-key-please-configure"
	}

	p.kvstore = kvstore.NewKVStore(p.client, encryptionKey)

	p.commandClient = command.NewCommandHandler(p.client, p)

	p.router = p.initRouter()

	p.updateSettingsHeader()

	return nil
}

func (p *Plugin) updateSettingsHeader() {
	siteURL := ""
	if cfg := p.API.GetConfig(); cfg.ServiceSettings.SiteURL != nil {
		siteURL = *cfg.ServiceSettings.SiteURL
	}

	redirectURI := fmt.Sprintf("%s/plugins/%s/api/v1/oauth/callback", siteURL, manifest.Id)

	header := fmt.Sprintf(
		"**Setup instructions:**\n"+
			"1. Enable the [Google Meet REST API](https://console.cloud.google.com/apis/library/meet.googleapis.com) for your Google Cloud project.\n"+
			"2. Create an [OAuth 2.0 Client ID](https://console.cloud.google.com/apis/credentials) (Web application type).\n"+
			"3. Add the following as an authorized redirect URI: `%s`\n"+
			"4. Enter the Client ID and Client Secret below.",
		redirectURI,
	)

	if manifest.SettingsSchema != nil {
		manifest.SettingsSchema.Header = header
	}
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	response, err := p.commandClient.Handle(args)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return response, nil
}
