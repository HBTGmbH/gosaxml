//go:build !amd64 && !arm64

package gosaxml

func indexSeparatorLong(buf []byte) int {
	r := indexSeparatorGeneric(buf[scalarPrefix:])
	if r < 0 {
		return -1
	}
	return scalarPrefix + r
}

func indexNonSpaceLong(buf []byte) int {
	r := indexNonSpaceGeneric(buf[scalarPrefix:])
	if r < 0 {
		return -1
	}
	return scalarPrefix + r
}
