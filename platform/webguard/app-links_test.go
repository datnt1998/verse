package webguard

import "testing"

func TestAppLinksRejectUnsafeConfiguration(t *testing.T) {
	for _, links := range []string{
		"viderse=https://evil.example.net", "other=https://viderse.example.com",
		"viderse=https://viderse.example.com/path", "viderse=http://viderse.example.com",
		"viderse=https://unlisted.example.com", "viderse=https://viderse.example.com,viderse=https://viderse.example.com",
	} {
		t.Run(links, func(t *testing.T) {
			_, err := envConfig(t, map[string]string{"PUBLIC_URL": "https://noverse.example.com", "SECURE_COOKIES": "true", "VERSE_COOKIE_DOMAIN": "example.com", "VERSE_APP_ORIGINS": "https://viderse.example.com", "VERSE_APP_LINKS": links})
			if err == nil {
				t.Fatal("unsafe VERSE_APP_LINKS accepted")
			}
		})
	}
}
