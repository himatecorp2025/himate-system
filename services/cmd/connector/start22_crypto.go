package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const start22EncryptionAlgorithm = "AES-256-GCM"

var start22KeyVersionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

type start22Keyring struct {
	ActiveVersion string
	Keys          map[string][]byte
}

type start22EncryptedEnvelope struct {
	Ciphertext  []byte
	DataNonce   []byte
	WrappedKey  []byte
	KeyNonce    []byte
	KeyVersion  string
}

func start22DecodeAES256Key(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("encryption key is required")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("encryption key must be standard base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must decode to exactly 32 bytes, got %d", len(key))
	}
	return key, nil
}

func start22LoadKeyringFromEnv() (start22Keyring, error) {
	activeVersion := strings.TrimSpace(os.Getenv("HIMATE_CONNECTOR_DATA_KEY_VERSION"))
	if activeVersion == "" {
		activeVersion = "v1"
	}
	if !start22KeyVersionPattern.MatchString(activeVersion) {
		return start22Keyring{}, errors.New("HIMATE_CONNECTOR_DATA_KEY_VERSION is invalid")
	}
	activeKey, err := start22DecodeAES256Key(os.Getenv("HIMATE_CONNECTOR_DATA_MASTER_KEY_B64"))
	if err != nil {
		return start22Keyring{}, fmt.Errorf("HIMATE_CONNECTOR_DATA_MASTER_KEY_B64: %w", err)
	}
	keys := map[string][]byte{activeVersion: activeKey}

	previousRaw := strings.TrimSpace(os.Getenv("HIMATE_CONNECTOR_DATA_PREVIOUS_KEYS_JSON"))
	if previousRaw != "" {
		var previous map[string]string
		if err := json.Unmarshal([]byte(previousRaw), &previous); err != nil {
			return start22Keyring{}, fmt.Errorf("HIMATE_CONNECTOR_DATA_PREVIOUS_KEYS_JSON must be a JSON object: %w", err)
		}
		for version, encoded := range previous {
			version = strings.TrimSpace(version)
			if !start22KeyVersionPattern.MatchString(version) || version == activeVersion {
				return start22Keyring{}, fmt.Errorf("invalid or duplicate previous key version %q", version)
			}
			key, err := start22DecodeAES256Key(encoded)
			if err != nil {
				return start22Keyring{}, fmt.Errorf("previous key %s: %w", version, err)
			}
			keys[version] = key
		}
	}
	return start22Keyring{ActiveVersion: activeVersion, Keys: keys}, nil
}

func start22GCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256 key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func start22RandomBytes(size int) ([]byte, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func start22RecordAAD(partnerID, environment, datasetKey, idempotencyKey string) []byte {
	return []byte(strings.Join([]string{
		"HIMATE", "START22", "DATA", "V1",
		strings.TrimSpace(partnerID),
		strings.TrimSpace(environment),
		strings.TrimSpace(datasetKey),
		strings.TrimSpace(idempotencyKey),
	}, "\x1f"))
}

func start22WrapAAD(aad []byte) []byte {
	out := make([]byte, 0, len(aad)+16)
	out = append(out, aad...)
	out = append(out, []byte("\x1fDEK\x1fV1")...)
	return out
}

func start22Zero(raw []byte) {
	for i := range raw {
		raw[i] = 0
	}
}

func (k start22Keyring) Encrypt(plaintext, aad []byte) (start22EncryptedEnvelope, error) {
	master, ok := k.Keys[k.ActiveVersion]
	if !ok || len(master) != 32 {
		return start22EncryptedEnvelope{}, errors.New("active START-22 encryption key is unavailable")
	}
	dek, err := start22RandomBytes(32)
	if err != nil {
		return start22EncryptedEnvelope{}, err
	}
	defer start22Zero(dek)

	dataGCM, err := start22GCM(dek)
	if err != nil {
		return start22EncryptedEnvelope{}, err
	}
	dataNonce, err := start22RandomBytes(dataGCM.NonceSize())
	if err != nil {
		return start22EncryptedEnvelope{}, err
	}
	ciphertext := dataGCM.Seal(nil, dataNonce, plaintext, aad)

	masterGCM, err := start22GCM(master)
	if err != nil {
		return start22EncryptedEnvelope{}, err
	}
	keyNonce, err := start22RandomBytes(masterGCM.NonceSize())
	if err != nil {
		return start22EncryptedEnvelope{}, err
	}
	wrappedKey := masterGCM.Seal(nil, keyNonce, dek, start22WrapAAD(aad))

	return start22EncryptedEnvelope{
		Ciphertext: ciphertext,
		DataNonce: dataNonce,
		WrappedKey: wrappedKey,
		KeyNonce: keyNonce,
		KeyVersion: k.ActiveVersion,
	}, nil
}

func (k start22Keyring) Decrypt(envelope start22EncryptedEnvelope, aad []byte) ([]byte, error) {
	master, ok := k.Keys[strings.TrimSpace(envelope.KeyVersion)]
	if !ok || len(master) != 32 {
		return nil, fmt.Errorf("START-22 encryption key version %q is unavailable", envelope.KeyVersion)
	}
	masterGCM, err := start22GCM(master)
	if err != nil {
		return nil, err
	}
	dek, err := masterGCM.Open(nil, envelope.KeyNonce, envelope.WrappedKey, start22WrapAAD(aad))
	if err != nil {
		return nil, errors.New("could not unwrap START-22 data key")
	}
	defer start22Zero(dek)

	dataGCM, err := start22GCM(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := dataGCM.Open(nil, envelope.DataNonce, envelope.Ciphertext, aad)
	if err != nil {
		return nil, errors.New("could not decrypt START-22 connector data")
	}
	return plaintext, nil
}

func (a *app) start22EncryptData(data map[string]any, partnerID, environment, datasetKey, idempotencyKey string) (start22EncryptedEnvelope, error) {
	plaintext := start22CanonicalJSON(data)
	if len(plaintext) == 0 {
		return start22EncryptedEnvelope{}, errors.New("could not canonicalize START-22 data")
	}
	return a.dataKeyring.Encrypt(plaintext, start22RecordAAD(partnerID, environment, datasetKey, idempotencyKey))
}

func (a *app) start22DecryptData(envelope start22EncryptedEnvelope, partnerID, environment, datasetKey, idempotencyKey string) (map[string]any, error) {
	plaintext, err := a.dataKeyring.Decrypt(envelope, start22RecordAAD(partnerID, environment, datasetKey, idempotencyKey))
	if err != nil {
		return nil, err
	}
	var data map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(plaintext)))
	if err := decoder.Decode(&data); err != nil {
		return nil, errors.New("decrypted START-22 payload is not valid JSON")
	}
	if data == nil {
		return nil, errors.New("decrypted START-22 payload is empty")
	}
	return data, nil
}
