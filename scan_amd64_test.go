package gosaxml

import (
	"math/rand"
	"testing"
)

// TestScannersAllSIMDModes runs the scanner cross-check once per SIMD path
// that this CPU actually supports, so that the SSE and AVX2 chunk loops are
// covered and not just whichever one the CPU happens to select.
func TestScannersAllSIMDModes(t *testing.T) {
	modes := []struct {
		name      string
		sse, avx2 bool
	}{{"generic", false, false}}
	if canUseSSE {
		modes = append(modes, struct {
			name      string
			sse, avx2 bool
		}{"sse", true, false})
	}
	if canUseAVX2 {
		modes = append(modes, struct {
			name      string
			sse, avx2 bool
		}{"avx2", true, true})
	}
	origSSE, origAVX2 := canUseSSE, canUseAVX2
	defer func() { canUseSSE, canUseAVX2 = origSSE, origAVX2 }()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			canUseSSE, canUseAVX2 = mode.sse, mode.avx2
			r := rand.New(rand.NewSource(1))
			for iter := 0; iter < 20000; iter++ {
				buf := make([]byte, r.Intn(300))
				for i := range buf {
					switch r.Intn(4) {
					case 0:
						buf[i] = byte(r.Intn(256))
					case 1:
						buf[i] = ' '
					default:
						buf[i] = 'a'
					}
				}
				if got, want := scanSeparator(buf), refIndexSeparator(buf); got != want {
					t.Fatalf("scanSeparator(%q) = %d, want %d", buf, got, want)
				}
				if got, want := scanNonSpace(buf), refIndexNonSpace(buf); got != want {
					t.Fatalf("scanNonSpace(%q) = %d, want %d", buf, got, want)
				}
			}
		})
	}
}
