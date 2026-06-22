package outbound

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"
	"unsafe"

	utls "github.com/metacubex/utls"
	aTLS "github.com/sagernet/sing/common/tls"
	"golang.org/x/crypto/hkdf"
)

type realityConfig struct {
	uClient   *utlsConfig
	publicKey []byte
	shortID   [8]byte
}

func newRealityConfig(opt tlsOptions) (aTLS.Config, error) {
	u, err := newUTLSConfig(opt)
	if err != nil {
		return nil, err
	}
	return &realityConfig{
		uClient:   u.(*utlsConfig),
		publicKey: opt.publicKey,
		shortID:   opt.shortID,
	}, nil
}

func (e *realityConfig) ServerName() string              { return e.uClient.ServerName() }
func (e *realityConfig) SetServerName(serverName string) { e.uClient.SetServerName(serverName) }
func (e *realityConfig) NextProtos() []string            { return e.uClient.NextProtos() }
func (e *realityConfig) SetNextProtos(nextProto []string) {
	e.uClient.SetNextProtos(nextProto)
}
func (e *realityConfig) Config() (*aTLS.STDConfig, error) {
	return nil, fmt.Errorf("reality has no std config")
}
func (e *realityConfig) Client(conn net.Conn) (aTLS.Conn, error) {
	return e.ClientHandshake(context.Background(), conn)
}
func (e *realityConfig) Clone() aTLS.Config {
	return &realityConfig{
		uClient:   e.uClient.Clone().(*utlsConfig),
		publicKey: append([]byte(nil), e.publicKey...),
		shortID:   e.shortID,
	}
}

func (e *realityConfig) ClientHandshake(ctx context.Context, conn net.Conn) (aTLS.Conn, error) {
	verifier := &realityVerifier{serverName: e.uClient.ServerName()}
	uConfig := e.uClient.cfg.Clone()
	uConfig.InsecureSkipVerify = true
	uConfig.SessionTicketsDisabled = true
	uConfig.VerifyPeerCertificate = verifier.VerifyPeerCertificate
	uConn := utls.UClient(conn, uConfig, e.uClient.id)
	verifier.UConn = uConn
	if err := uConn.BuildHandshakeState(); err != nil {
		return nil, err
	}
	for _, extension := range uConn.Extensions {
		if ce, ok := extension.(*utls.SupportedCurvesExtension); ok {
			filtered := ce.Curves[:0]
			for _, curveID := range ce.Curves {
				if curveID != utls.X25519MLKEM768 {
					filtered = append(filtered, curveID)
				}
			}
			ce.Curves = filtered
		}
		if ks, ok := extension.(*utls.KeyShareExtension); ok {
			filtered := ks.KeyShares[:0]
			for _, share := range ks.KeyShares {
				if share.Group != utls.X25519MLKEM768 {
					filtered = append(filtered, share)
				}
			}
			ks.KeyShares = filtered
		}
	}
	if err := uConn.BuildHandshakeState(); err != nil {
		return nil, err
	}
	if len(uConfig.NextProtos) > 0 {
		for _, extension := range uConn.Extensions {
			if alpnExtension, isALPN := extension.(*utls.ALPNExtension); isALPN {
				alpnExtension.AlpnProtocols = uConfig.NextProtos
				break
			}
		}
	}
	hello := uConn.HandshakeState.Hello
	hello.SessionId = make([]byte, 32)
	copy(hello.Raw[39:], hello.SessionId)
	now := time.Now().Unix()
	binary.BigEndian.PutUint64(hello.SessionId, uint64(now))
	hello.SessionId[0] = 1
	hello.SessionId[1] = 8
	hello.SessionId[2] = 1
	binary.BigEndian.PutUint32(hello.SessionId[4:], uint32(now))
	copy(hello.SessionId[8:], e.shortID[:])
	publicKey, err := ecdh.X25519().NewPublicKey(e.publicKey)
	if err != nil {
		return nil, err
	}
	keyShareKeys := uConn.HandshakeState.State13.KeyShareKeys
	if keyShareKeys == nil || keyShareKeys.Ecdhe == nil {
		return nil, fmt.Errorf("reality: missing ecdhe key")
	}
	authKey, err := keyShareKeys.Ecdhe.ECDH(publicKey)
	if err != nil || authKey == nil {
		return nil, fmt.Errorf("reality: ecdh failed")
	}
	verifier.authKey = authKey
	_, err = hkdf.New(sha256.New, authKey, hello.Random[:20], []byte("REALITY")).Read(authKey)
	if err != nil {
		return nil, err
	}
	aesBlock, _ := aes.NewCipher(authKey)
	aesGcmCipher, _ := cipher.NewGCM(aesBlock)
	aesGcmCipher.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
	copy(hello.Raw[39:], hello.SessionId)
	if err := uConn.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	if !verifier.verified {
		return nil, fmt.Errorf("reality verification failed")
	}
	return &realityConnWrapper{UConn: uConn}, nil
}

type realityVerifier struct {
	*utls.UConn
	serverName string
	authKey    []byte
	verified   bool
}

func (c *realityVerifier) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	p, _ := reflect.TypeOf(c.Conn).Elem().FieldByName("peerCertificates")
	certs := *(*[]*x509.Certificate)(unsafe.Pointer(uintptr(unsafe.Pointer(c.Conn)) + p.Offset))
	if len(certs) == 0 {
		return fmt.Errorf("reality: no peer certificate")
	}
	if pub, ok := certs[0].PublicKey.(ed25519.PublicKey); ok {
		h := hmac.New(sha512.New, c.authKey)
		h.Write(pub)
		if bytes.Equal(h.Sum(nil), certs[0].Signature) {
			c.verified = true
			return nil
		}
	}
	opts := x509.VerifyOptions{DNSName: c.serverName, Intermediates: x509.NewCertPool()}
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}
	if _, err := certs[0].Verify(opts); err != nil {
		return err
	}
	return nil
}

type realityConnWrapper struct {
	*utls.UConn
}

func (c *realityConnWrapper) NetConn() net.Conn { return c.UConn }
func (c *realityConnWrapper) ConnectionState() tls.ConnectionState {
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
func (c *realityConnWrapper) Upstream() any { return c.UConn }

var _ aTLS.ConfigCompat = (*realityConfig)(nil)
