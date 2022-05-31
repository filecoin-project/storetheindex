package loadgen

import (
	"io"
	"math/rand"
)

type pr struct {
	rand.Source
}

func newPseudoRandReaderFrom(src rand.Source) io.Reader {
	return &pr{src}
}

func (r *pr) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(r.Int63())
	}
	return len(p), nil
}
