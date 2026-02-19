package cache

import (
	"errors"
	"testing"
)

func TestEncodeValueRespectsLimitEqualsLen(t *testing.T) {
	out, err := encodeValue(CompressionNone, 3, []byte("abc"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "abc" {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestDecodeValuePassThrough(t *testing.T) {
	in := []byte("plain")
	out, err := decodeValue(in)
	if err != nil {
		t.Fatalf("decode err: %v", err)
	}
	if string(out) != "plain" {
		t.Fatalf("expected passthrough")
	}
}

func TestDecodeValueShortInput(t *testing.T) {
	out, err := decodeValue([]byte("tiny"))
	if err != nil {
		t.Fatalf("decode short err: %v", err)
	}
	if string(out) != "tiny" {
		t.Fatalf("expected passthrough on short input")
	}
}

func TestDecodeValueSnappyUnsupported(t *testing.T) {
	in := append([]byte("CMP1"), 's')
	in = append(in, []byte{0x00}...)
	if _, err := decodeValue(in); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("expected unsupported codec error, got %v", err)
	}
}

func TestEncodeValueGzipEarlySizeCheck(t *testing.T) {
	if _, err := encodeValue(CompressionGzip, 1, []byte("toolong")); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestDecodeValueGzipSuccess(t *testing.T) {
	encoded, err := encodeValue(CompressionGzip, 0, []byte("ok"))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := decodeValue(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(decoded) != "ok" {
		t.Fatalf("unexpected decode value: %s", string(decoded))
	}
}
