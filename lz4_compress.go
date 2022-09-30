package gw_cache

import (
	"bytes"
	"github.com/pierrec/lz4"
	"io"
)

func lz4Compress(in []byte) ([]byte, error) {
	r := bytes.NewReader(in)
	w := &bytes.Buffer{}
	zw := lz4.NewWriter(w)
	_, err := io.Copy(zw, r)
	if err != nil {
		return nil, err
	}
	// Closing is *very* important
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func lz4Decompress(in []byte) ([]byte, error) {
	r := bytes.NewReader(in)
	w := &bytes.Buffer{}
	zr := lz4.NewReader(r)
	_, err := io.Copy(w, zr)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}
