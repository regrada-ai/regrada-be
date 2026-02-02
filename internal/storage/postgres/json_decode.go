package postgres

import (
	"encoding/base64"
	"encoding/json"
)

func decodeJSONField(raw []byte, target any) error {
	if len(raw) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, target); err == nil {
		return nil
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return err
	}

	decoded, ok := decodeBase64(encoded)
	if ok {
		if err := json.Unmarshal(decoded, target); err == nil {
			return nil
		}
	}

	return json.Unmarshal([]byte(encoded), target)
}

func decodeBase64(value string) ([]byte, bool) {
	if value == "" {
		return nil, false
	}

	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err == nil {
		return decoded, true
	}

	decoded, err = base64.StdEncoding.DecodeString(value)
	if err == nil {
		return decoded, true
	}

	return nil, false
}
