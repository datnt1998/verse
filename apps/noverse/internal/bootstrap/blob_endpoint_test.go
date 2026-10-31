package bootstrap

import "testing"

func TestPrivateStorageEndpoint(t *testing.T) {
	for _, test := range []struct {
		endpoint string
		want     bool
	}{
		{"", false}, {"relative/path", false}, {"ftp://garage", false}, {"://bad", false},
		{"https://s3.amazonaws.com", false}, {"https://acct.r2.cloudflarestorage.com", false}, {"http://s3.amazonaws.com", true},
		{"https://garage", true}, {"https://10.0.0.1", true}, {"https://172.16.0.1", true}, {"https://172.31.255.255", true}, {"https://172.32.0.1", false}, {"https://192.168.0.1", true}, {"https://127.0.0.1", true}, {"https://169.254.10.1", true},
		{"https://8.8.8.8", false}, {"https://[fd00::1]", true}, {"https://[fe80::1]", true}, {"https://[::1]", true}, {"https://[::ffff:192.168.1.1]", true}, {"https://[::ffff:8.8.8.8]", false}, {"https://[::ffff:808:808]", false}, {"https://[2606:4700:4700::1111]", false},
	} {
		t.Run(test.endpoint, func(t *testing.T) {
			if got := privateEndpoint(test.endpoint); got != test.want {
				t.Fatalf("privateEndpoint(%q)=%t, want %t", test.endpoint, got, test.want)
			}
		})
	}
}
