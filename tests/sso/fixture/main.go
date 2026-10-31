// ssoproxy terminates TLS for the hosted-cookie rehearsal and forwards plain
// HTTP to the loopback app behind each virtual host, exactly like the deployed
// Traefik boundary does. It generates a throwaway CA at startup, mints one
// certificate covering every routed host, and answers READY on stdout once it
// listens.
//
//	-ssoproxy -listen 127.0.0.1:8443 -route noverse.verse.test=127.0.0.1:18100 \
//	    -route viderse.verse.test=127.0.0.1:18101 -route khac.other.test=127.0.0.1:18100
//
// The CA is never written to disk and never installed anywhere: the test
// browser runs with ignoreHTTPSErrors instead of widening system trust.
// The proxy owns the X-Forwarded-* boundary: inbound forwarding headers are
// deleted and rewritten from the real peer, never passed through.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

type routesFlag map[string]string

func (f *routesFlag) String() string {
	var out []string
	for host, target := range *f {
		out = append(out, host+"="+target)
	}
	return strings.Join(out, ",")
}

func (f *routesFlag) Set(value string) error {
	host, target, ok := strings.Cut(value, "=")
	if !ok || host == "" || target == "" {
		return fmt.Errorf("route cần dạng host=127.0.0.1:port, nhận %q", value)
	}
	if net.ParseIP(host) != nil || strings.ContainsAny(host, " /?#") {
		return fmt.Errorf("host %q không hợp lệ", host)
	}
	if _, port, err := net.SplitHostPort(target); err != nil || port == "" {
		return fmt.Errorf("target %q không phải host:port", target)
	}
	(*f)[strings.ToLower(host)] = target
	return nil
}

func newCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "verse-sso-fixture CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(4 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func issueServerCert(ca *x509.Certificate, caKey *ecdsa.PrivateKey, hosts []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hosts[0]},
		DNSNames:     hosts,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(4 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: tpl}, nil
}

func main() {
	listen := flag.String("listen", "127.0.0.1:8443", "địa chỉ TLS lắng nghe (loopback)")
	mkaccountsDir := flag.String("mkaccounts", "", "tuyệt đối: ghi fixture web-accounts.json vào thư mục này rồi thoát (không proxy)")
	routes := routesFlag{}
	flag.Var(&routes, "route", "host=127.0.0.1:port, lặp cho mỗi vhost")
	flag.Parse()
	if *mkaccountsDir != "" {
		if len(routes) > 0 {
			log.Fatal("-mkaccounts không đi cùng -route")
		}
		if err := mkaccounts(*mkaccountsDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(routes) == 0 {
		log.Fatal("cần ít nhất một -route")
	}
	// The proxy binds loopback only: a bound public interface would expose the
	// plain-HTTP apps behind it to the LAN.
	if host, _, err := net.SplitHostPort(*listen); err != nil || !net.ParseIP(host).IsLoopback() {
		log.Fatalf("-listen %q phải là loopback host:port", *listen)
	}

	ca, caKey, err := newCA()
	if err != nil {
		log.Fatal(err)
	}
	hosts := make([]string, 0, len(routes))
	for host := range routes {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	cert, err := issueServerCert(ca, caKey, hosts)
	if err != nil {
		log.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		target, ok := routes[host]
		if !ok {
			http.Error(w, "vhost không được định tuyến", http.StatusBadGateway)
			return
		}
		remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = "http"
				req.URL.Host = target
				// The proxy owns the forwarding boundary: inbound
				// X-Forwarded-* is attacker-controlled and is rewritten, never
				// trusted or appended to.
				req.Header.Del("X-Forwarded-For")
				req.Header.Del("X-Forwarded-Proto")
				req.Header.Del("X-Forwarded-Host")
				req.Header.Del("Forwarded")
				req.Header.Set("X-Forwarded-Proto", "https")
				req.Header.Set("X-Forwarded-For", remoteIP)
			},
			FlushInterval: -1,
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
				http.Error(w, "upstream không phản hồi", http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    *listen,
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.ServeTLS(ln, "", "") }()
	fmt.Printf("READY %s\n", *listen)

	stop := make(chan os.Signal, 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		shutdownDone := make(chan struct{})
		go func() {
			_ = server.Close()
			close(shutdownDone)
		}()
		select {
		case <-shutdownDone:
		case <-time.After(3 * time.Second):
		}
	}
}
