package kvstore

import "time"

// OAuth2Token represents an OAuth2 token.
type OAuth2Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

type KVStore interface {
	StoreOAuth2Token(userID string, token *OAuth2Token) error
	GetOAuth2Token(userID string) (*OAuth2Token, error)
	DeleteOAuth2Token(userID string) error
	StoreOAuth2State(state string, userID string) error
	GetAndDeleteOAuth2State(state string) (string, error)
}
