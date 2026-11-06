package gosaxml

import (
	"math/rand"
	"testing"
)

// scanSeparator / scanNonSpace mirror exactly how the decoder composes the
// head scan with the vectorized long scan.
func scanSeparator(buf []byte) int {
	k := indexSeparatorHead(buf)
	if k < 0 && len(buf) > scalarPrefix {
		k = indexSeparatorLong(buf)
	}
	return k
}

func scanNonSpace(buf []byte) int {
	k := indexNonSpaceHead(buf)
	if k < 0 && len(buf) > scalarPrefix {
		k = indexNonSpaceLong(buf)
	}
	return k
}

// referenceSeparator is the specification of the name separator set.
func referenceSeparator(b byte) bool {
	switch b {
	case '\t', '\n', '\r', ' ', '/', ':', '=', '>', '?':
		return true
	}
	return false
}

func refIndexSeparator(buf []byte) int {
	for i, b := range buf {
		if referenceSeparator(b) {
			return i
		}
	}
	return -1
}

func refIndexNonSpace(buf []byte) int {
	for i, b := range buf {
		if b > ' ' {
			return i
		}
	}
	return -1
}

// TestScannersExhaustive checks every byte value at every position of a
// buffer spanning several vector widths, for every buffer length.
func TestScannersExhaustive(t *testing.T) {
	const size = 3*scalarPrefix + 17
	for v := 0; v < 256; v++ {
		for pos := 0; pos < size; pos++ {
			sepBuf := make([]byte, size)
			spcBuf := make([]byte, size)
			for i := range sepBuf {
				sepBuf[i] = 'a'
				spcBuf[i] = ' '
			}
			sepBuf[pos] = byte(v)
			spcBuf[pos] = byte(v)
			for l := 0; l <= size; l++ {
				if got, want := scanSeparator(sepBuf[:l]), refIndexSeparator(sepBuf[:l]); got != want {
					t.Fatalf("scanSeparator(v=%#x pos=%d len=%d) = %d, want %d", v, pos, l, got, want)
				}
				if got, want := indexSeparatorGeneric(sepBuf[:l]), refIndexSeparator(sepBuf[:l]); got != want {
					t.Fatalf("indexSeparatorGeneric(v=%#x pos=%d len=%d) = %d, want %d", v, pos, l, got, want)
				}
				if got, want := scanNonSpace(spcBuf[:l]), refIndexNonSpace(spcBuf[:l]); got != want {
					t.Fatalf("scanNonSpace(v=%#x pos=%d len=%d) = %d, want %d", v, pos, l, got, want)
				}
				if got, want := indexNonSpaceGeneric(spcBuf[:l]), refIndexNonSpace(spcBuf[:l]); got != want {
					t.Fatalf("indexNonSpaceGeneric(v=%#x pos=%d len=%d) = %d, want %d", v, pos, l, got, want)
				}
			}
		}
	}
}

func TestScannersRandom(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for iter := 0; iter < 50000; iter++ {
		buf := make([]byte, r.Intn(400))
		for i := range buf {
			buf[i] = byte(r.Intn(256))
		}
		if got, want := scanSeparator(buf), refIndexSeparator(buf); got != want {
			t.Fatalf("scanSeparator(%q) = %d, want %d", buf, got, want)
		}
		if got, want := scanNonSpace(buf), refIndexNonSpace(buf); got != want {
			t.Fatalf("scanNonSpace(%q) = %d, want %d", buf, got, want)
		}
	}
}

// TestScannersMostlyWhitespace exercises the long/vectorized paths with
// inputs that stay in the scanned class for a long time.
func TestScannersMostlyWhitespace(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for iter := 0; iter < 20000; iter++ {
		buf := make([]byte, r.Intn(400))
		for i := range buf {
			if r.Intn(20) == 0 {
				buf[i] = byte(r.Intn(256))
			} else {
				buf[i] = ' '
			}
		}
		if got, want := scanNonSpace(buf), refIndexNonSpace(buf); got != want {
			t.Fatalf("scanNonSpace(%q) = %d, want %d", buf, got, want)
		}
		for i := range buf {
			if buf[i] == ' ' {
				buf[i] = 'a'
			}
		}
		if got, want := scanSeparator(buf), refIndexSeparator(buf); got != want {
			t.Fatalf("scanSeparator(%q) = %d, want %d", buf, got, want)
		}
	}
}
