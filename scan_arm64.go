package gosaxml

// indexSeparatorNEON returns the index of the first XML name separator in buf,
// or -1. Implemented in neon_arm64.s.
//
//go:noescape
func indexSeparatorNEON(buf []byte) int

// indexNonSpaceNEON returns the index of the first byte in buf greater than
// ' ', or -1. Implemented in neon_arm64.s.
//
//go:noescape
func indexNonSpaceNEON(buf []byte) int

// indexSeparatorLong scans buf beyond the scalar prefix already covered by
// indexSeparatorHead. Callers must only reach it for len(buf) > scalarPrefix.
func indexSeparatorLong(buf []byte) int {
	r := indexSeparatorNEON(buf[scalarPrefix:])
	if r < 0 {
		return -1
	}
	return scalarPrefix + r
}

// indexNonSpaceLong scans buf beyond the scalar prefix already covered by
// indexNonSpaceHead. Callers must only reach it for len(buf) > scalarPrefix.
func indexNonSpaceLong(buf []byte) int {
	r := indexNonSpaceNEON(buf[scalarPrefix:])
	if r < 0 {
		return -1
	}
	return scalarPrefix + r
}
