package bootstrap

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
)

func TestConfigEncryptionStrict(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{41}, 32))
	for i, raw := range []string{"", "invalid-base64", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{41}, 31)), base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{41}, 33)), good + "\n", " " + good} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Setenv("CONFIG_ENCRYPTION_KEY", raw)
			t.Setenv("STORE_DRIVER", "file")
			_, err := StorageConfigFromEnv()
			if err == nil {
				t.Fatal("malformed encryption key accepted at startup")
			}
			if raw != "" && strings.Contains(err.Error(), raw) {
				t.Fatal("invalid encryption key leaked in startup error")
			}
		})
	}
	t.Setenv("CONFIG_ENCRYPTION_KEY", good)
	cipher, err := encryptionFromEnv()
	if err != nil || cipher.aead == nil || cipher.aead.NonceSize() != 12 || !strings.HasPrefix(cipher.id, "aes256gcm-v1:") {
		t.Fatalf("valid AES-256-GCM config rejected: %v", err)
	}
}
