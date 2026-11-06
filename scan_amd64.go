package gosaxml

import "github.com/klauspost/cpuid/v2"

var canUseSSE = cpuid.CPU.Has(cpuid.SSE2) && cpuid.CPU.Has(cpuid.BMI1)
var canUseAVX2 = canUseSSE && cpuid.CPU.Has(cpuid.AVX2)

// indexSeparatorLong scans buf beyond the scalar prefix already covered by
// indexSeparatorHead. Callers must only reach it for len(buf) > scalarPrefix.
//
// The SIMD helpers always consume a whole vector, so they are only fed full
// 32-byte blocks; the remainder is finished off scalar.
func indexSeparatorLong(buf []byte) int {
	i := scalarPrefix
	if canUseAVX2 {
		for len(buf)-i >= 32 {
			r := int(seperator32(buf[i:]))
			if r < 32 {
				return i + r
			}
			i += 32
		}
	}
	r := indexSeparatorGeneric(buf[i:])
	if r < 0 {
		return -1
	}
	return i + r
}

// indexNonSpaceLong scans buf beyond the scalar prefix already covered by
// indexNonSpaceHead. Callers must only reach it for len(buf) > scalarPrefix.
func indexNonSpaceLong(buf []byte) int {
	i := scalarPrefix
	if canUseAVX2 {
		for len(buf)-i >= 32 {
			r := int(onlySpaces32(buf[i:]))
			if r < 32 {
				return i + r
			}
			i += 32
		}
	}
	if canUseSSE {
		for len(buf)-i >= 16 {
			r := int(onlySpaces16(buf[i:]))
			if r < 16 {
				return i + r
			}
			i += 16
		}
	}
	r := indexNonSpaceGeneric(buf[i:])
	if r < 0 {
		return -1
	}
	return i + r
}
