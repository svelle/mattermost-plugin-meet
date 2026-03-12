package kvstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"

	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"
)

const (
	oauthTokenPrefix = "oauth_token_"
	oauthStatePrefix = "oauth_state_"
	stateTTLSeconds  = 600 // 10 minutes
)

type Client struct {
	client        *pluginapi.Client
	encryptionKey string
}

func NewKVStore(client *pluginapi.Client, encryptionKey string) KVStore {
	return &Client{
		client:        client,
		encryptionKey: encryptionKey,
	}
}

func (kv *Client) StoreOAuth2Token(userID string, token *OAuth2Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return errors.Wrap(err, "failed to marshal token")
	}

	encrypted, err := encrypt(data, kv.encryptionKey)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt token")
	}

	_, err = kv.client.KV.Set(oauthTokenPrefix+userID, encrypted)
	if err != nil {
		return errors.Wrap(err, "failed to store token")
	}
	return nil
}

func (kv *Client) GetOAuth2Token(userID string) (*OAuth2Token, error) {
	var encrypted []byte
	err := kv.client.KV.Get(oauthTokenPrefix+userID, &encrypted)
	if err != nil {
		// If the key doesn't exist or can't be unmarshalled, treat as not connected
		return nil, nil
	}
	if len(encrypted) == 0 {
		return nil, nil
	}

	data, err := decrypt(encrypted, kv.encryptionKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt token")
	}

	var token OAuth2Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal token")
	}
	return &token, nil
}

func (kv *Client) DeleteOAuth2Token(userID string) error {
	err := kv.client.KV.Delete(oauthTokenPrefix + userID)
	if err != nil {
		return errors.Wrap(err, "failed to delete token")
	}
	return nil
}

func (kv *Client) StoreOAuth2State(state string, userID string) error {
	_, err := kv.client.KV.Set(oauthStatePrefix+state, []byte(userID), pluginapi.SetExpiry(stateTTLSeconds))
	if err != nil {
		return errors.Wrap(err, "failed to store OAuth state")
	}
	return nil
}

func (kv *Client) GetAndDeleteOAuth2State(state string) (string, error) {
	var userID []byte
	err := kv.client.KV.Get(oauthStatePrefix+state, &userID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get OAuth state")
	}
	if userID == nil {
		return "", errors.New("OAuth state not found or expired")
	}

	_ = kv.client.KV.Delete(oauthStatePrefix + state)

	return string(userID), nil
}

func deriveKey(encryptionKey string) []byte {
	hash := sha256.Sum256([]byte(encryptionKey))
	return hash[:]
}

func encrypt(data []byte, encryptionKey string) ([]byte, error) {
	key := deriveKey(encryptionKey)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

func decrypt(data []byte, encryptionKey string) ([]byte, error) {
	key := deriveKey(encryptionKey)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
