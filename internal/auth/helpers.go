package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// computeSecretHash computes the secret hash for Cognito authentication
// Required when the app client has a client secret
func computeSecretHash(username, clientID, clientSecret string) string {
	message := username + clientID
	key := []byte(clientSecret)

	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
