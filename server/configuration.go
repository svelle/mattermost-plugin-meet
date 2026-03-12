package main

import (
	"reflect"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-meet/server/store/kvstore"
)

type configuration struct {
	GoogleClientID     string `json:"GoogleClientID"`
	GoogleClientSecret string `json:"GoogleClientSecret"`
	EncryptionKey      string `json:"EncryptionKey"`
}

func (c *configuration) Clone() *configuration {
	clone := *c
	return &clone
}

func (c *configuration) IsValid() error {
	if c.GoogleClientID == "" {
		return errors.New("Google Client ID is not configured")
	}
	if c.GoogleClientSecret == "" {
		return errors.New("Google Client Secret is not configured")
	}
	if c.EncryptionKey == "" {
		return errors.New("Encryption Key is not configured")
	}
	return nil
}

func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

func (p *Plugin) OnConfigurationChange() error {
	configuration := new(configuration)

	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)
	if p.client != nil {
		if configuration.IsValid() == nil {
			p.setKVStore(kvstore.NewKVStore(p.client, configuration.EncryptionKey))
		} else {
			p.setKVStore(nil)
		}
	}
	p.updateSettingsHeader()

	return nil
}
