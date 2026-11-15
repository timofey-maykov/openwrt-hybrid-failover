package subscription

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type responseTransport func(*http.Request) (*http.Response, error)

func (f responseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSubscriptionFallback(t *testing.T) {
	for _, failure := range []string{"network", "HTTP 403", "block page", "body read"} {
		t.Run(failure, func(t *testing.T) {
			var calls []string
			client := func(name string, fail bool) *http.Client {
				return &http.Client{Transport: responseTransport(func(r *http.Request) (*http.Response, error) {
					calls = append(calls, name)
					if fail && failure == "network" {
						return nil, errors.New("blocked")
					}
					body := "hy2://password@192.0.2.20:443#Working"
					code := 200
					if fail {
						body = "<html>Blocked</html>"
						if failure == "HTTP 403" {
							code = 403
						}
					}
					var reader io.ReadCloser = io.NopCloser(strings.NewReader(body))
					if fail && failure == "body read" {
						reader = brokenBody{}
					}
					return &http.Response{StatusCode: code, Header: http.Header{"Profile-Title": []string{"Panel"}}, Body: reader}, nil
				})}
			}
			f := &Fetcher{Identity: ClientIdentity{UserAgent: "test-client"}, HTTP: client("direct", true), FallbackClients: func(context.Context) []*http.Client {
				return []*http.Client{client("dead tunnel", true), client("working tunnel", false), client("unused", false)}
			}}
			res, err := f.Fetch([]string{"https://panel.example/sub/private-token"})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Links) != 1 || res.Sources[0].Title != "Panel" {
				t.Fatalf("result: %+v", res)
			}
			if !reflect.DeepEqual(calls, []string{"direct", "dead tunnel", "working tunnel"}) {
				t.Fatal(calls)
			}
		})
	}
}

type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (brokenBody) Close() error             { return nil }

func TestSuccessfulDirectSubscriptionDoesNotDiscoverTunnels(t *testing.T) {
	f := NewFetcher()
	f.FallbackClients = func(context.Context) []*http.Client { t.Fatal("unnecessary tunnel discovery"); return nil }
	if _, err := f.FetchURLs([]string{serve(t, "hy2://password@192.0.2.20:443")}); err != nil {
		t.Fatal(err)
	}
}

func TestFailedFallbackPreservesPanelReasonAndMasksToken(t *testing.T) {
	f := NewFetcher()
	f.FallbackClients = func(context.Context) []*http.Client {
		return []*http.Client{{Transport: responseTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("tunnel unavailable") })}}
	}
	_, err := f.FetchURLs([]string{panelServer(t, nil, 0, 403) + "/secret-token"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error: %v", err)
	}
}

func TestOversizedSubscriptionIsNotAcceptedPartially(t *testing.T) {
	f := NewFetcher()
	f.FallbackClients = nil
	body := "hy2://password@192.0.2.20:443\n" + strings.Repeat("#", 4<<20)
	_, err := f.FetchURLs([]string{serve(t, body)})
	if err == nil || !strings.Contains(err.Error(), "exceeds 4 MiB") {
		t.Fatalf("error: %v", err)
	}
}

func TestRefreshSharesCancellationAcrossURLs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := 0
	f := &Fetcher{HTTP: &http.Client{Transport: responseTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		cancel() // first URL consumed the caller's remaining budget
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("hy2://password@192.0.2.20:443"))}, nil
	})}, FallbackClients: func(context.Context) []*http.Client { t.Fatal("discovery after cancellation"); return nil }}
	_, err := f.FetchContext(ctx, []string{"https://panel.example/one", "https://panel.example/two"})
	if !errors.Is(err, context.Canceled) || requests != 1 {
		t.Fatalf("requests=%d error=%v", requests, err)
	}
}
