package gosaxml

// These helpers each classify one fixed-width vector and return the index of
// the first matching byte, or the vector width if there is none.
// bytes.IndexByte already provides an optimized scan for plain byte searches,
// so only the two XML-specific character classes are implemented here.

//go:noescape
func onlySpaces16([]uint8) byte

//go:noescape
func onlySpaces32([]uint8) byte

//go:noescape
func seperator32([]uint8) byte
