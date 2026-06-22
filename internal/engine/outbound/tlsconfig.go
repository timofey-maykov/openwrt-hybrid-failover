package outbound

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	utls "github.com/metacubex/utls"
	aTLS "github.com/sagernet/sing/common/tls"
)

type tlsOptions struct {
	enabled    bool
	serverName string
	insecure   bool
	fingerprint string
	reality    bool
	publicKey  []byte
	shortID    [8]byte
}

func tlsFromFields(fields map[string]any, serverHost string) (tlsOptions, error) {
	opt := tlsOptions{serverName: serverHost}
	raw, _ := fields["tls"].(map[string]any)
	if raw == nil {
		return opt, nil
	}
	if v, ok := raw["enabled"].(bool); ok {
		opt.enabled = v
	}
	if sn, ok := raw["server_name"].(string); ok && sn != "" {
		opt.serverName = sn
	}
	if v, ok := raw["insecure"].(bool); ok {
		opt.insecure = v
	}
	if utlsMap, ok := raw["utls"].(map[string]any); ok {
		if fp, ok := utlsMap["fingerprint"].(string); ok {
			opt.fingerprint = fp
		}
	}
	if reality, ok := raw["reality"].(map[string]any); ok {
		if en, ok := reality["enabled"].(bool); ok && en {
			opt.reality = true
			pbk, _ := reality["public_key"].(string)
			key, err := base64.RawURLEncoding.DecodeString(pbk)
			if err != nil {
				return opt, fmt.Errorf("reality public_key: %w", err)
			}
			if len(key) != 32 {
				return opt, fmt.Errorf("reality public_key: invalid length")
			}
			opt.publicKey = key
			sid, _ := reality["short_id"].(string)
			n, err := hex.Decode(opt.shortID[:], []byte(sid))
			if err != nil {
				return opt, fmt.Errorf("reality short_id: %w", err)
			}
			if n > 8 {
				return opt, fmt.Errorf("reality short_id: too long")
			}
		}
	}
	if opt.enabled && opt.serverName == "" {
		opt.serverName = serverHost
	}
	return opt, nil
}

func buildTLSConfig(fields map[string]any, serverHost string) (aTLS.Config, error) {
	opt, err := tlsFromFields(fields, serverHost)
	if err != nil {
		return nil, err
	}
	if !opt.enabled {
		return nil, nil
	}
	if opt.reality {
		return newRealityConfig(opt)
	}
	if opt.fingerprint != "" {
		return newUTLSConfig(opt)
	}
	return newSTDConfig(opt), nil
}

type stdTLSConfig struct {
	cfg tls.Config
}

func newSTDConfig(opt tlsOptions) aTLS.Config {
	cfg := tls.Config{
		ServerName:         opt.serverName,
		InsecureSkipVerify: opt.insecure,
		NextProtos:         []string{"h2", "http/1.1"},
	}
	return &stdTLSConfig{cfg: cfg}
}

func (c *stdTLSConfig) ServerName() string                     { return c.cfg.ServerName }
func (c *stdTLSConfig) SetServerName(serverName string)        { c.cfg.ServerName = serverName }
func (c *stdTLSConfig) NextProtos() []string                   { return c.cfg.NextProtos }
func (c *stdTLSConfig) SetNextProtos(nextProto []string)       { c.cfg.NextProtos = nextProto }
func (c *stdTLSConfig) Config() (*aTLS.STDConfig, error)       { return &c.cfg, nil }
func (c *stdTLSConfig) Client(conn net.Conn) (aTLS.Conn, error) {
	return tls.Client(conn, &c.cfg), nil
}
func (c *stdTLSConfig) Clone() aTLS.Config {
	clone := c.cfg
	return &stdTLSConfig{cfg: clone}
}

type utlsConfig struct {
	cfg *utls.Config
	id  utls.ClientHelloID
}

func newUTLSConfig(opt tlsOptions) (aTLS.Config, error) {
	id, err := utlsFingerprint(opt.fingerprint)
	if err != nil {
		return nil, err
	}
	cfg := &utls.Config{
		ServerName:         opt.serverName,
		InsecureSkipVerify: opt.insecure,
		NextProtos:         []string{"h2", "http/1.1"},
	}
	return &utlsConfig{cfg: cfg, id: id}, nil
}

func (c *utlsConfig) ServerName() string              { return c.cfg.ServerName }
func (c *utlsConfig) SetServerName(serverName string) { c.cfg.ServerName = serverName }
func (c *utlsConfig) NextProtos() []string            { return c.cfg.NextProtos }
func (c *utlsConfig) SetNextProtos(nextProto []string) {
	c.cfg.NextProtos = nextProto
}
func (c *utlsConfig) Config() (*aTLS.STDConfig, error) { return nil, fmt.Errorf("utls has no std config") }
func (c *utlsConfig) Client(conn net.Conn) (aTLS.Conn, error) {
	return &utlsConnWrap{UConn: utls.UClient(conn, c.cfg.Clone(), c.id)}, nil
}

type utlsConnWrap struct {
	*utls.UConn
}

func (c *utlsConnWrap) NetConn() net.Conn { return c.UConn }
func (c *utlsConnWrap) ConnectionState() tls.ConnectionState {
	state := c.UConn.ConnectionState()
	return tls.ConnectionState{
		Version:                     state.Version,
		HandshakeComplete:           state.HandshakeComplete,
		DidResume:                   state.DidResume,
		CipherSuite:                 state.CipherSuite,
		NegotiatedProtocol:          state.NegotiatedProtocol,
		NegotiatedProtocolIsMutual:  state.NegotiatedProtocolIsMutual,
		ServerName:                  state.ServerName,
		PeerCertificates:            state.PeerCertificates,
		VerifiedChains:              state.VerifiedChains,
		SignedCertificateTimestamps: state.SignedCertificateTimestamps,
		OCSPResponse:                state.OCSPResponse,
		TLSUnique:                   state.TLSUnique,
	}
}
func (c *utlsConnWrap) Upstream() any { return c.UConn }
func (c *utlsConfig) Clone() aTLS.Config {
	return &utlsConfig{cfg: c.cfg.Clone(), id: c.id}
}

func utlsFingerprint(name string) (utls.ClientHelloID, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "chrome":
		return utls.HelloChrome_Auto, nil
	case "firefox":
		return utls.HelloFirefox_Auto, nil
	case "edge":
		return utls.HelloEdge_Auto, nil
	case "safari":
		return utls.HelloSafari_Auto, nil
	case "ios":
		return utls.HelloIOS_Auto, nil
	case "android":
		return utls.HelloAndroid_11_OkHttp, nil
	default:
		return utls.ClientHelloID{}, fmt.Errorf("unknown uTLS fingerprint %q", name)
	}
}

type stdConnWrapper struct {
	*tls.Conn
}

func (c *stdConnWrapper) NetConn() net.Conn { return c.Conn }
func (c *stdConnWrapper) Upstream() any     { return c.Conn }

func dialTLS(ctx context.Context, conn net.Conn, cfg aTLS.Config) (net.Conn, error) {
	if cfg == nil {
		return conn, nil
	}
	tlsConn, err := aTLS.ClientHandshake(ctx, conn, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func rootCAs() *x509.CertPool {
	pool, _ := x509.SystemCertPool()
	return pool
}
