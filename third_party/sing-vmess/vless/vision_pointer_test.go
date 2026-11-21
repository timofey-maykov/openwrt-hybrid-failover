package vless

import (
	"bytes"
	"reflect"
	"runtime"
	"testing"
	"unsafe"
)

func TestVisionTLSBufferPointers(t *testing.T) {
	type buffers struct {
		prefix   [17]byte
		input    bytes.Reader
		rawInput bytes.Buffer
	}
	makePointers := func() (*bytes.Reader, *bytes.Buffer) {
		owner := &buffers{input: *bytes.NewReader([]byte("plaintext"))}
		owner.rawInput.WriteString("encrypted")
		input, rawInput, err := visionTLSBuffers(reflect.TypeOf(*owner), unsafe.Pointer(owner))
		if err != nil {
			t.Fatal(err)
		}
		if input != &owner.input || rawInput != &owner.rawInput {
			t.Fatal("incorrect TLS field offsets")
		}
		return input, rawInput
	}
	input, rawInput := makePointers()
	// Only the interior pointers retain the owner after this point.
	runtime.GC()
	if got := input.Len(); got != len("plaintext") {
		t.Fatalf("input len=%d", got)
	}
	if got := rawInput.String(); got != "encrypted" {
		t.Fatalf("raw input=%q", got)
	}
}

func TestVisionTLSBufferLayoutValidation(t *testing.T) {
	for _, owner := range []any{
		&struct{ input bytes.Reader }{},
		&struct{ rawInput bytes.Buffer }{},
		&struct {
			input    []byte
			rawInput bytes.Buffer
		}{},
		&struct {
			input    bytes.Reader
			rawInput *bytes.Buffer
		}{},
	} {
		v := reflect.ValueOf(owner)
		if _, _, err := visionTLSBuffers(v.Type().Elem(), v.UnsafePointer()); err == nil {
			t.Fatalf("accepted incompatible TLS layout %T", owner)
		}
	}
	if _, _, err := visionTLSBuffers(nil, nil); err == nil {
		t.Fatal("accepted nil layout")
	}
}
