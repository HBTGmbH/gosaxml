package gosaxml

//go:noescape
func openAngleBracket16([]uint8) byte

//go:noescape
func onlySpaces16([]uint8) byte

func onlySpacesUntil16(slice []uint8, n byte) bool {
	return onlySpaces16(slice) >= n
}

//go:noescape
func openAngleBracket32([]uint8) byte

//go:noescape
func onlySpaces32([]uint8) byte

func onlySpacesUntil32(slice []uint8, n byte) bool {
	return onlySpaces32(slice) >= n
}
