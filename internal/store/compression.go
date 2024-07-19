package store

import (
	"bytes"
	"compress/zlib"
	"io"
)

// compressData compresses the given data using the zlib compression algorithm.
func compressData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// decompressData decompresses the given data using the zlib compression algorithm.
func decompressData(data []byte) ([]byte, error) {
	b := bytes.NewReader(data)
	r, err := zlib.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
