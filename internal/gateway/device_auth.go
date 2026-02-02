package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// DeviceIdentity represents a generated device identity.
type DeviceIdentity struct {
	ID         string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateDeviceIdentity generates a new Ed25519 key pair and device ID.
func GenerateDeviceIdentity() (*DeviceIdentity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	id := ComputeDeviceID(pub)
	return &DeviceIdentity{
		ID:         id,
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// ComputeDeviceID calculates the SHA256 fingerprint of the raw public key.
func ComputeDeviceID(pubKey ed25519.PublicKey) string {
	hash := sha256.Sum256(pubKey)
	return hex.EncodeToString(hash[:])
}

// BuildDeviceAuthPayload constructs the payload string for signing.
// Format: version|deviceId|clientId|clientMode|role|scopes|signedAtMs|token(|nonce)
func BuildDeviceAuthPayload(deviceID, clientID, clientMode, role string, scopes []string, signedAtMs int64, token, nonce string) string {
	version := "v1"
	if nonce != "" {
		version = "v2"
	}

	parts := []string{
		version,
		deviceID,
		clientID,
		clientMode,
		role,
		strings.Join(scopes, ","),
		fmt.Sprintf("%d", signedAtMs),
		token, // token is empty string if not provided
	}

	if version == "v2" {
		parts = append(parts, nonce)
	}

	return strings.Join(parts, "|")
}

// SignDevicePayload signs the payload with the private key and returns base64url encoded signature.
func SignDevicePayload(privKey ed25519.PrivateKey, payload string) string {
	sig := ed25519.Sign(privKey, []byte(payload))
	return base64.RawURLEncoding.EncodeToString(sig)
}

// PublicKeyToBase64Url returns the base64url encoded raw public key.
func PublicKeyToBase64Url(pubKey ed25519.PublicKey) string {
	return base64.RawURLEncoding.EncodeToString(pubKey)
}

// PEM export helpers (if needed for storage later)
func EncodePrivateKeyPEM(privKey ed25519.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, err
	}
	// In Go, usually using pem.Encode
	// but we'll stick to raw keys in memory for now
	return b, nil
}
