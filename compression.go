package cache

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"

	"github.com/goforj/cache/cachecore"
)

// CompressionCodec represents a value compression algorithm.
type CompressionCodec = cachecore.CompressionCodec

const (
	CompressionNone   = cachecore.CompressionNone
	CompressionGzip   = cachecore.CompressionGzip
	CompressionSnappy = cachecore.CompressionSnappy
)

var (
	compressMagic = []byte("CMP1")

	ErrValueTooLarge      = errors.New("cache: value exceeds max size")
	ErrUnsupportedCodec   = errors.New("cache: unsupported compression codec")
	ErrCorruptCompression = errors.New("cache: corrupt compressed payload")
)

func encodeValue(codec CompressionCodec, max int, value []byte) ([]byte, error) {
	if max > 0 && len(value) > max {
		return nil, ErrValueTooLarge
	}
	switch codec {
	case CompressionNone:
		return value, nil
	case CompressionGzip:
		var buf bytes.Buffer
		buf.Write(compressMagic)
		_ = buf.WriteByte('g')
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		if _, err := zw.Write(value); err != nil {
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		out := buf.Bytes()
		if max > 0 && len(out) > max {
			return nil, ErrValueTooLarge
		}
		return out, nil
	case CompressionSnappy:
		return nil, ErrUnsupportedCodec
	default:
		return nil, ErrUnsupportedCodec
	}
}

func decodeValue(in []byte) ([]byte, error) {
	if len(in) < len(compressMagic)+1 {
		return in, nil
	}
	if !bytes.Equal(in[:len(compressMagic)], compressMagic) {
		return in, nil
	}
	codec := in[len(compressMagic)]
	payload := in[len(compressMagic)+1:]
	switch codec {
	case 'g':
		gr, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, ErrCorruptCompression
		}
		defer gr.Close()
		out, err := io.ReadAll(gr)
		if err != nil {
			return nil, ErrCorruptCompression
		}
		return out, nil
	case 's':
		return nil, ErrUnsupportedCodec
	default:
		return nil, ErrUnsupportedCodec
	}
}
