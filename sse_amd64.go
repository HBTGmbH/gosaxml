package gosaxml

//go:noescape
func openAngleBracket16([]uint8) uint16

func findFirstOpenAngleBracket16(slice []uint8) int {
	return int(openAngleBracket16(slice))
}

//go:noescape
func onlySpaces16([]uint8) uint16

func onlySpacesUntil16(slice []uint8, n int) bool {
	return onlySpaces16(slice)<<(16-n) == 0
}

//go:noescape
func openAngleBracket32([]uint8) int

func findFirstOpenAngleBracket32(slice []uint8) int {
	return openAngleBracket32(slice)
}

//go:noescape
func onlySpaces32([]uint8) uint32

func onlySpacesUntil32(slice []uint8, n int) bool {
	return onlySpaces32(slice)<<(32-n) == 0
}
