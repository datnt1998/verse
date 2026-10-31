package blobstore

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestS3RejectsResolvedMetadataAddresses(t *testing.T) {
	transport := s3HTTPClient.Transport.(*http.Transport)
	for _, host := range []string{"169.254.169.254", "169.254.1.1", "[fe80::1]", "[fd00:ec2::254]", "[::ffff:169.254.169.254]"} {
		t.Run(host, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			conn, err := transport.DialContext(ctx, "tcp", host+":80")
			if conn != nil {
				conn.Close()
				t.Fatal("metadata connection established")
			}
			if err == nil || !strings.Contains(err.Error(), "storage endpoint denied") {
				t.Fatalf("metadata address not refused before connect: %v", err)
			}
		})
	}
	// Resolve a harmless-looking endpoint name to IMDS through a controlled DNS
	// server. The check must happen on the resolved address, not the URL hostname.
	dns, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer dns.Close()
	var queries atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		for {
			n, addr, e := dns.ReadFrom(buf)
			if e != nil {
				return
			}
			var msg dnsmessage.Message
			if msg.Unpack(buf[:n]) != nil {
				continue
			}
			queries.Add(1)
			msg.Header.Response = true
			msg.Header.Authoritative = true
			msg.Header.RecursionAvailable = true
			for _, q := range msg.Questions {
				if q.Type == dnsmessage.TypeA {
					msg.Answers = append(msg.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 0}, Body: &dnsmessage.AResource{A: [4]byte{169, 254, 169, 254}}})
				}
			}
			out, e := msg.Pack()
			if e != nil {
				return
			}
			if _, e = dns.WriteTo(out, addr); e != nil {
				return
			}
		}
	}()
	old := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", dns.LocalAddr().String())
	}}
	defer func() { net.DefaultResolver = old; dns.Close(); <-done }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := transport.DialContext(ctx, "tcp", "innocent.example.test:80")
	if conn != nil {
		conn.Close()
		t.Fatal("DNS metadata connection established")
	}
	if queries.Load() == 0 || err == nil || !strings.Contains(err.Error(), "storage endpoint denied") {
		t.Fatalf("DNS metadata address not checked: queries=%d err=%v", queries.Load(), err)
	}
}

func TestS3DoesNotFollowRedirects(t *testing.T) {
	if s3HTTPClient.CheckRedirect == nil {
		t.Fatal("S3 client follows redirects without a policy")
	}
	err := s3HTTPClient.CheckRedirect(&http.Request{}, nil)
	if err != http.ErrUseLastResponse {
		t.Fatalf("S3 redirects enabled: %v", err)
	}
}

func TestS3ConditionalCleanupPreservesDeleteError(t *testing.T) {
	store, err := NewS3(S3Config{Endpoint: "http://s3.invalid", Region: "garage", Bucket: "test", AccessKeyID: "test", SecretAccessKey: "secret", AllowPlainHTTP: true, ForcePathStyle: true, HTTPClient: &http.Client{Transport: s3TestTransport(func(r *http.Request) (*http.Response, error) {
		status, body := 200, ""
		if r.Method == "DELETE" {
			status = 403
			body = "<Error><Code>AccessDenied</Code></Error>"
		}
		if r.Header.Get("If-None-Match") == "*" {
			status = 412
			body = "<Error><Code>PreconditionFailed</Code></Error>"
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	key := "staging/test/a1111111-1111-4111-8111-111111111111"
	_, err = store.Put(context.Background(), key, strings.NewReader("payload"), PutOptions{})
	if err == nil || !strings.Contains(err.Error(), "delete conditional probe") {
		t.Fatalf("probe cleanup source lost: %v", err)
	}
	if store.ConditionalPutSupport() != "unknown" {
		t.Fatal("failed cleanup marked capability successful")
	}
}
