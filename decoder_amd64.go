package gosaxml

import "github.com/klauspost/cpuid/v2"

var canUseSSE = cpuid.CPU.Has(cpuid.SSE2) && cpuid.CPU.Has(cpuid.BMI1)
var canUseAVX2 = canUseSSE && cpuid.CPU.Has(cpuid.AVX2)

func init() {
	if canUseAVX2 {
		simdWidth = 32
	} else if canUseSSE {
		simdWidth = 16
	}
}

func (thiz *decoder) skipWhitespaces(b byte) (byte, error) {
	if canUseAVX2 {
		return thiz.skipWhitespacesAVX2(b)
	} else if canUseSSE {
		return thiz.skipWhitespacesSSE(b)
	}
	return thiz.skipWhitespacesGeneric(b)
}

func (thiz *decoder) skipWhitespacesAVX2(b byte) (byte, error) {
	if !isWhitespace(b) {
		return b, nil
	}
	for {
		for thiz.w > thiz.r {
			sidx, isWhole := clampToBuf(onlySpaces32, 32, thiz.rb[thiz.r:thiz.w])
			_, err := thiz.discard(sidx)
			if err != nil {
				return 0, err
			}
			if !isWhole {
				newB, err := thiz.readByte()
				if err != nil {
					return 0, err
				}
				return newB, nil
			}
		}
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return 0, err
		}
	}
}

func (thiz *decoder) skipWhitespacesSSE(b byte) (byte, error) {
	if !isWhitespace(b) {
		return b, nil
	}
	for {
		j := thiz.r
		c := 0
		for thiz.w > thiz.r+c {
			sidx, isWhole := clampToBuf(onlySpaces16, 16, thiz.rb[j+c:thiz.w])
			c += int(sidx)
			if !isWhole {
				_, err := thiz.discard(c)
				if err != nil {
					return 0, err
				}
				newB, err := thiz.readByte()
				if err != nil {
					return 0, err
				}
				return newB, nil
			}
		}
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return 0, err
		}
	}
}

func (thiz *decoder) decodeText(t *Token) (bool, error) {
	if canUseAVX2 {
		return thiz.decodeTextAVX2(t)
	} else if canUseSSE {
		return thiz.decodeTextSSE(t)
	}
	return thiz.decodeTextGeneric(t)
}

func (thiz *decoder) decodeTextSSE(t *Token) (bool, error) {
	i := len(thiz.bb)
	onlyWhitespaces := true
	for {
		j := thiz.r
		c := 0
		for thiz.w > thiz.r+c {
			sidx := openAngleBracket16(thiz.rb[j+c : thiz.w])
			onlyWhitespaces = onlyWhitespaces && onlySpaces16(thiz.rb[j+c:thiz.w]) >= sidx
			c += int(sidx)
			if sidx != 16 {
				_, err := thiz.discard(c)
				if err != nil {
					return false, err
				}
				if onlyWhitespaces && !thiz.preserveWhitespaces[thiz.top] {
					return true, nil
				}
				thiz.bb = append(thiz.bb, thiz.rb[j:j+c]...)
				t.Kind = TokenTypeTextElement
				t.ByteData = thiz.bb[i:len(thiz.bb)]
				return false, nil
			}
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return false, err
		}
	}
}

func (thiz *decoder) decodeTextAVX2(t *Token) (bool, error) {
	i := len(thiz.bb)
	onlyWhitespaces := true
	for {
		j := thiz.r
		c := 0
		for thiz.w > thiz.r+c {
			sidx, isWhole := clampToBuf(openAngleBracket32, 32, thiz.rb[j+c:thiz.w])
			onlyWhitespaces = onlyWhitespaces && int(onlySpaces32(thiz.rb[j+c:thiz.w])) >= sidx
			c += int(sidx)
			if !isWhole {
				_, err := thiz.discard(c)
				if err != nil {
					return false, err
				}
				if onlyWhitespaces && !thiz.preserveWhitespaces[thiz.top] {
					return true, nil
				}
				thiz.bb = append(thiz.bb, thiz.rb[j:j+c]...)
				t.Kind = TokenTypeTextElement
				t.ByteData = thiz.bb[i:len(thiz.bb)]
				return false, nil
			}
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return false, err
		}
	}
}

func (thiz *decoder) readSimpleName() ([]byte, byte, error) {
	if canUseAVX2 {
		return thiz.readSimpleNameAVX()
	}
	return thiz.readSimpleNameGeneric()
}

func (thiz *decoder) readSimpleNameAVX() ([]byte, byte, error) {
	i := len(thiz.bb)
	for {
		j := thiz.r
		c := 0
		for thiz.w > thiz.r+c {
			sidx, isWhole := clampToBuf(seperator32, 32, thiz.rb[j+c:thiz.w])
			c += sidx
			if !isWhole {
				_, err := thiz.discard(c + 1)
				if err != nil {
					return nil, 0, err
				}
				thiz.bb = append(thiz.bb, thiz.rb[j:j+c]...)
				return thiz.bb[i:len(thiz.bb)], thiz.rb[j+c], nil
			}
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return nil, 0, err
		}
	}
}

// clampToBuf adapts a fixed-width SIMD scanner to arbitrary-length slices.
// It clamps the returned index when buf is shorter than vectorSize and reports
// whether the scan covered the entire provided buf (no early terminator found).
func clampToBuf(vectorFn func([]byte) byte, vectorSize int, buf []byte) (int, bool) {
	res := int(vectorFn(buf))
	if res >= len(buf) {
		return len(buf), true
	}
	return res, res == vectorSize
}
