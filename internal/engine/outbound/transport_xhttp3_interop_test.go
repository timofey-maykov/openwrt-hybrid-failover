package outbound

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// HF_XRAY_BINARY enables a real Xray server compatibility check, without
// downloading or executing external binaries during the normal test suite.
func TestXHTTP3XrayInterop(t *testing.T) {
	for _, mode := range []string{"auto", "stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) { runXHTTPXrayInterop(t, "h3", mode) })
	}
}
func TestXHTTPXrayInteropH1H2(t *testing.T) {
	for _, proto := range []string{"http/1.1", "h2"} {
		for _, mode := range []string{"auto", "stream-one", "stream-up", "packet-up"} {
			t.Run(proto+"/"+mode, func(t *testing.T) {
				if proto == "http/1.1" && mode != "auto" && mode != "packet-up" {
					t.Skip("Xray direct HTTP/1.1 listener does not enable full-duplex uploads")
				}
				runXHTTPXrayInterop(t, proto, mode)
			})
		}
	}
}
func runXHTTPXrayInterop(t *testing.T, proto, mode string, options ...string) {

	binary := os.Getenv("HF_XRAY_BINARY")
	if binary == "" {
		t.Skip("set HF_XRAY_BINARY to test against Xray")
	}
	cert := selfSigned(t)
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()
	config := map[string]any{
		"log": map[string]any{"loglevel": "info"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": port, "protocol": "vless",
			"settings": map[string]any{"clients": []any{map[string]any{"id": vlessTestUUID}}, "decryption": "none"},
			"streamSettings": map[string]any{"network": "xhttp", "security": "tls",
				"tlsSettings": map[string]any{"alpn": []string{proto}, "certificates": []any{map[string]any{
					"certificate": strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}))), "\n"),
					"key":         strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))), "\n"),
				}}}, "xhttpSettings": map[string]any{"path": "/interop", "mode": "auto"}},
		}},
		"outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.1/32"}}}}}},
	}
	realityQuery := ""
	for _, option := range options {
		if option == "REALITY" {
			key, err := ecdh.X25519().GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			fallback := &http.Server{Handler: http.NotFoundHandler(), TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2", "http/1.1"}}}
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = fallback.Close() })
			go func() { _ = fallback.ServeTLS(ln, "", "") }()
			stream := config["inbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
			stream["security"] = "reality"
			delete(stream, "tlsSettings")
			stream["realitySettings"] = map[string]any{"target": ln.Addr().String(), "dest": ln.Addr().String(), "serverNames": []string{"localhost"}, "privateKey": base64.RawURLEncoding.EncodeToString(key.Bytes()), "shortIds": []string{"0123456789abcdef"}}
			realityQuery = "&security=reality&fp=chrome&pbk=" + base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()) + "&sid=0123456789abcdef&alpn=h3"
		}
	}
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.json")
	body, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configFile, body, 0600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "xray.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "run", "-config", configFile)
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = log.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			t.Log(string(data))
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, _ := os.ReadFile(logPath)
		if strings.Contains(string(data), " started") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Xray did not start: %s", data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tcp.Close() })
	go func() {
		for {
			c, err := tcp.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = udp.Close() })
	go func() {
		b := make([]byte, 1500)
		for {
			n, a, err := udp.ReadFrom(b)
			if err != nil {
				return
			}
			_, _ = udp.WriteTo(b[:n], a)
		}
	}()
	link := fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=xhttp&alpn=%s&path=%%2Finterop&mode=%s", vlessTestUUID, port, proto, mode)
	parsed, _ := url.Parse(link)
	query := parsed.Query()
	for _, option := range append(options, realityQuery) {
		if option == "REALITY" {
			continue
		}
		v, err := url.ParseQuery(strings.TrimPrefix(option, "&"))
		if err != nil {
			t.Fatal(err)
		}
		for k, vals := range v {
			query[k] = vals
		}
	}
	parsed.RawQuery = query.Encode()
	h := newTestVLESSLink(t, parsed.String())
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c, err := h.DialTCP(ctx, "tcp", tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// Both the VLESS header and payload cross a real Xray QUIC/HTTP/3 listener.
	done := make(chan error, 1)
	go func() {
		if _, err := c.Write([]byte("tcp-through-xray")); err != nil {
			done <- err
			return
		}
		b := make([]byte, len("tcp-through-xray"))
		_, err := io.ReadFull(c, b)
		if err == nil && string(b) != "tcp-through-xray" {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("TCP echo timed out")
	}
	u, err := h.DialUDP(ctx, "udp", udp.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()
	go func() {
		if _, err := u.WriteTo([]byte("udp-through-xray"), nil); err != nil {
			done <- err
			return
		}
		b := make([]byte, 1500)
		n, _, err := u.ReadFrom(b)
		if err == nil && string(b[:n]) != "udp-through-xray" {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("UDP echo timed out")
	}
}

func TestXHTTPXrayTLSFragmentation(t *testing.T) {
	for _, mode := range []string{"auto", "stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) {
			runXHTTPXrayInterop(t, "h2", mode, "&fp=firefox&fm="+url.QueryEscape(helloFragment))
		})
	}
}

func TestXHTTPXrayReality(t *testing.T) {
	for _, mode := range []string{"auto", "stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) { runXHTTPXrayInterop(t, "h2", mode, "REALITY") })
	}
}
