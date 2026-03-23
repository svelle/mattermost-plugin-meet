package main

import "fmt"

func (p *Plugin) GetConnectURL() string {
	return p.getOAuth2ConnectURL()
}

func (p *Plugin) DisconnectUser(userID string) error {
	store, err := p.getOAuthKVStore()
	if err != nil {
		return err
	}

	if err := store.DeleteOAuth2Token(userID); err != nil {
		return fmt.Errorf("failed to delete user token: %w", err)
	}

	return nil
}

func (p *Plugin) IsUserConnected(userID string) (bool, error) {
	store, err := p.getOAuthKVStore()
	if err != nil {
		return false, err
	}

	token, err := store.GetOAuth2Token(userID)
	if err != nil {
		return false, err
	}

	return token != nil, nil
}

func (p *Plugin) IsUserAdmin(userID string) (bool, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false, fmt.Errorf("failed to get user: %w", appErr)
	}

	return user.IsSystemAdmin(), nil
}
