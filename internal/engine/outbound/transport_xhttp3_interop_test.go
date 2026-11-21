package outbound

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
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
				"tlsSettings": map[string]any{"alpn": []string{"h3"}, "certificates": []any{map[string]any{
					"certificate": strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}))), "\n"),
					"key":         strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))), "\n"),
				}}}, "xhttpSettings": map[string]any{"path": "/interop", "mode": "auto"}},
		}},
		"outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.1/32"}}}}}},
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
	h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=xhttp&alpn=h3&path=%%2Finterop&mode=stream-one", vlessTestUUID, port))
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
