package gosaxml

// The NEON scanners finish a partial final block by reloading the last 16
// bytes of their input, which relies on the scalar head scan having covered
// at least that many bytes. This fails to compile if scalarPrefix ever drops
// below 16.
const _ uint = scalarPrefix - 16

// indexSeparatorNEON returns the index of the first XML name separator in
// buf[from:], relative to the start of buf, or -1. It requires len(buf) >= 16
// and buf[:from] to contain no separator. Implemented in neon_arm64.s.
//
//go:noescape
func indexSeparatorNEON(buf []byte, from int) int

// indexNonSpaceNEON returns the index of the first byte greater than ' ' in
// buf[from:], relative to the start of buf, or -1. It requires len(buf) >= 16
// and buf[:from] to contain no such byte. Implemented in neon_arm64.s.
//
//go:noescape
func indexNonSpaceNEON(buf []byte, from int) int

// indexSeparatorLong scans buf beyond the scalar prefix already covered by
// indexSeparatorHead. Callers must only reach it for len(buf) > scalarPrefix.
func indexSeparatorLong(buf []byte) int {
	return indexSeparatorNEON(buf, scalarPrefix)
}

// indexNonSpaceLong scans buf beyond the scalar prefix already covered by
// indexNonSpaceHead. Callers must only reach it for len(buf) > scalarPrefix.
func indexNonSpaceLong(buf []byte) int {
	return indexNonSpaceNEON(buf, scalarPrefix)
}
