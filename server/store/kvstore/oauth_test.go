package kvstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "test-encryption-key"
	plaintext := []byte("hello, world!")

	encrypted, err := encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := decrypt(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDecrypt_DifferentKeys(t *testing.T) {
	plaintext := []byte("secret data")

	encrypted, err := encrypt(plaintext, "key1")
	require.NoError(t, err)

	_, err = decrypt(encrypted, "key2")
	assert.Error(t, err, "decrypting with a different key should fail")
}

func TestEncryptDecrypt_EmptyData(t *testing.T) {
	key := "test-key"

	encrypted, err := encrypt([]byte{}, key)
	require.NoError(t, err)

	decrypted, err := decrypt(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, []byte{}, decrypted)
}

func TestEncrypt_ProducesDifferentCiphertext(t *testing.T) {
	key := "test-key"
	plaintext := []byte("same data")

	enc1, err := encrypt(plaintext, key)
	require.NoError(t, err)

	enc2, err := encrypt(plaintext, key)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "encrypting same data twice should produce different ciphertext due to random nonce")
}

func TestDecrypt_CiphertextTooShort(t *testing.T) {
	_, err := decrypt([]byte{1, 2}, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestDeriveKey_Deterministic(t *testing.T) {
	key1 := deriveKey("my-secret")
	key2 := deriveKey("my-secret")
	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 32, "derived key should be 32 bytes (AES-256)")
}

func TestDeriveKey_DifferentInputs(t *testing.T) {
	key1 := deriveKey("key-a")
	key2 := deriveKey("key-b")
	assert.NotEqual(t, key1, key2)
}
