package clash

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSwitchProxySendsJSONBody(t *testing.T) {
	var gotMethod, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cli := New(srv.URL, 0)
	if err := cli.SwitchProxy(context.Background(), "glob-out", "glob-1-out"); err != nil {
		t.Fatalf("SwitchProxy: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method: got %q want PUT", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type: got %q want application/json", gotContentType)
	}
	if gotBody != `{"name":"glob-1-out"}` {
		t.Fatalf("body: got %q", gotBody)
	}
	if !strings.Contains(srv.URL, "127.0.0.1") {
		t.Fatal("expected test server URL")
	}
}
