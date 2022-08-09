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
