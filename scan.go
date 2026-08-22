package gosaxml

import "bytes"

// nameSeparators marks every byte that terminates an XML name.
// Having a full 256-entry table lets the compiler drop the bounds
// check on the lookup, which matters because this table is consulted
// for every single byte of every name in the input.
var nameSeparators = func() [256]uint8 {
	var t [256]uint8
	t['\t'] = 1
	t['\n'] = 1
	t['\r'] = 1
	t[' '] = 1
	t['/'] = 1
	t[':'] = 1
	t['='] = 1
	t['>'] = 1
	t['?'] = 1
	return t
}()

// scalarPrefix is the number of leading bytes that the *Head scanners look at
// before the caller falls back to the vectorized *Long scanners.
//
// XML names and whitespace runs almost always terminate well within this many
// bytes. For those, the inlined scalar loop below is considerably cheaper than
// a call into hand-written assembly (which cannot be inlined, and whose
// vector-to-general-register round trip costs more than scanning a handful of
// bytes). Only genuinely long runs are worth vectorizing, and there the fixed
// cost of the scalar prefix disappears in the noise.
//
// The split is deliberately expressed as two functions rather than one: a
// wrapper containing the fallback call would exceed the inliner's budget and
// so would put a function call back into the hot path.
const scalarPrefix = 24

// indexSeparatorHead returns the index of the first name separator within the
// first scalarPrefix bytes of buf, or -1 if there is none. When it returns -1
// and len(buf) > scalarPrefix, the caller must consult indexSeparatorLong.
func indexSeparatorHead(buf []byte) int {
	head := buf
	if len(head) > scalarPrefix {
		head = head[:scalarPrefix]
	}
	// Two lookups per iteration: they are independent, so the loads overlap,
	// and only one branch is spent on the pair.
	i := 0
	for ; i+1 < len(head); i += 2 {
		if nameSeparators[head[i]]|nameSeparators[head[i+1]] != 0 {
			if nameSeparators[head[i]] != 0 {
				return i
			}
			return i + 1
		}
	}
	if i < len(head) && nameSeparators[head[i]] != 0 {
		return i
	}
	return -1
}

// indexNonSpaceHead returns the index of the first byte greater than ' '
// within the first scalarPrefix bytes of buf, or -1 if there is none. When it
// returns -1 and len(buf) > scalarPrefix, the caller must consult
// indexNonSpaceLong.
func indexNonSpaceHead(buf []byte) int {
	head := buf
	if len(head) > scalarPrefix {
		head = head[:scalarPrefix]
	}
	for i := 0; i < len(head); i++ {
		if head[i] > ' ' {
			return i
		}
	}
	return -1
}

// indexSeparatorGeneric is the portable implementation of the separator scan.
func indexSeparatorGeneric(buf []byte) int {
	for i := 0; i < len(buf); i++ {
		if nameSeparators[buf[i]] != 0 {
			return i
		}
	}
	return -1
}

// indexNonSpaceGeneric is the portable implementation of the whitespace scan.
func indexNonSpaceGeneric(buf []byte) int {
	for i := 0; i < len(buf); i++ {
		if buf[i] > ' ' {
			return i
		}
	}
	return -1
}

// quotePrefix is the analogue of scalarPrefix for the plain byte searches
// (attribute value terminators). The fallback there is bytes.IndexByte, whose
// call is cheaper than the assembly scanners', so the prefix is shorter.
const quotePrefix = 16

// indexByteHead returns the index of c within the first n bytes of buf,
// or -1 if it does not occur there.
func indexByteHead(buf []byte, c byte, n int) int {
	if len(buf) < n {
		n = len(buf)
	}
	for i := 0; i < n; i++ {
		if buf[i] == c {
			return i
		}
	}
	return -1
}

// indexOfAngle returns the index of the next '<' in buf, or -1.
// Character data is frequently just a handful of bytes (a number, a date, a
// short label), so the head of the run is scanned inline before falling back
// to the assembly-implemented bytes.IndexByte.
func indexOfAngle(buf []byte) int {
	if k := indexByteHead(buf, '<', quotePrefix); k >= 0 {
		return k
	}
	if len(buf) <= quotePrefix {
		return -1
	}
	k := bytes.IndexByte(buf[quotePrefix:], '<')
	if k < 0 {
		return -1
	}
	return quotePrefix + k
}
