#include "textflag.h"

// NOTE: this file must assemble with the oldest Go toolchain the module
// supports (see go.mod). That assembler knows only a subset of NEON, so the
// obvious spellings are unavailable: there is no VCMHI/VCMGT (unsigned and
// signed compares are emulated with VUMIN + VCMEQ), and no VSHRN (the
// per-lane results are condensed into a scalar syndrome with the
// VAND/VADDP/VADDP sequence that the runtime's own IndexByte uses).

// Nibble-lookup tables (http://0x80.pl/articles/simd-byte-lookup.html) for the
// set of bytes that terminate an XML name:
//
//     '\t' 09, '\n' 0A, '\r' 0D, ' ' 20, '/' 2F, ':' 3A, '=' 3D, '>' 3E, '?' 3F
//
// A byte b is in the set iff sepLo[b&0xF] & sepHi[b>>4] != 0. One bit is
// assigned per high nibble occurring in the set: bit 0 -> 0x0_, bit 1 -> 0x2_,
// bit 2 -> 0x3_. Bytes >= 0x80 select an all-zero high-nibble entry and so
// never match, which is what UTF-8 continuation bytes inside names need.
//
// sepLo, by index: 0:02  9:01  A:05  D:05  E:04  F:06  (all others 0)
DATA ·sepLo<>+0(SB)/8, $0x0000000000000002
DATA ·sepLo<>+8(SB)/8, $0x0604050000050100
GLOBL ·sepLo<>(SB), NOPTR+RODATA, $16

// sepHi, by index: 0:01  2:02  3:04  (all others 0)
DATA ·sepHi<>+0(SB)/8, $0x0000000004020001
DATA ·sepHi<>+8(SB)/8, $0x0000000000000000
GLOBL ·sepHi<>(SB), NOPTR+RODATA, $16

// The same separator set as a bitmap over the bytes 0x00..0x3F, used by the
// scalar tail loop: bits 9, 10, 13, 32, 47, 58, 61, 62, 63.
#define SEPBITS $0xE400800100002600

// Per-lane bit selector used to condense a vector of 0x00/0xFF lanes into a
// scalar syndrome: 0x40100401 = (1<<0) + (4<<8) + (16<<16) + (64<<24), so that
// two rounds of pairwise addition leave two syndrome bits per input byte.
#define LANEBITS $0x40100401

// func indexSeparatorNEON(buf []byte) int
//
// Returns the index of the first XML name separator in buf, or -1.
TEXT ·indexSeparatorNEON(SB), NOSPLIT, $0-32
	MOVD	buf_base+0(FP), R0
	MOVD	buf_len+8(FP), R1
	MOVD	R0, R3                  // R3 = base, to derive the index from

	CMP	$16, R1
	BLO	tail

	MOVD	$·sepLo<>(SB), R2
	VLD1	(R2), [V16.B16]
	MOVD	$·sepHi<>(SB), R2
	VLD1	(R2), [V17.B16]
	VMOVI	$15, V18.B16
	MOVD	LANEBITS, R5
	VMOV	R5, V19.S4

loop16:
	VLD1	(R0), [V0.B16]
	VAND	V18.B16, V0.B16, V1.B16 // low nibbles
	VUSHR	$4, V0.B16, V2.B16      // high nibbles
	VTBL	V1.B16, [V16.B16], V1.B16
	VTBL	V2.B16, [V17.B16], V2.B16
	VAND	V2.B16, V1.B16, V1.B16  // non-zero lane == separator
	VCMTST	V1.B16, V1.B16, V1.B16  // 0xFF where non-zero
	VAND	V19.B16, V1.B16, V1.B16
	VADDP	V1.B16, V1.B16, V2.B16
	VADDP	V2.B16, V2.B16, V2.B16
	VMOV	V2.D[0], R4
	CBNZ	R4, found16
	ADD	$16, R0, R0
	SUB	$16, R1, R1
	CMP	$16, R1
	BHS	loop16

tail:
	CBZ	R1, notfound
	MOVD	SEPBITS, R7

tailloop:
	MOVBU.P	1(R0), R5
	CMP	$64, R5
	BHS	tailnext
	LSR	R5, R7, R6
	AND	$1, R6, R6
	CBNZ	R6, tailfound

tailnext:
	SUBS	$1, R1, R1
	BNE	tailloop

notfound:
	MOVD	$-1, R4
	MOVD	R4, ret+24(FP)
	RET

found16:
	RBIT	R4, R4
	CLZ	R4, R4
	LSR	$1, R4, R4              // two syndrome bits per input byte
	SUB	R3, R0, R6
	ADD	R6, R4, R4
	MOVD	R4, ret+24(FP)
	RET

tailfound:
	SUB	$1, R0, R0
	SUB	R3, R0, R4
	MOVD	R4, ret+24(FP)
	RET

// func indexNonSpaceNEON(buf []byte) int
//
// Returns the index of the first byte in buf greater than ' ' (i.e. the first
// byte that is not XML whitespace), or -1 if buf is all whitespace.
TEXT ·indexNonSpaceNEON(SB), NOSPLIT, $0-32
	MOVD	buf_base+0(FP), R0
	MOVD	buf_len+8(FP), R1
	MOVD	R0, R3

	CMP	$16, R1
	BLO	tail

	VMOVI	$33, V16.B16            // ' ' + 1
	MOVD	LANEBITS, R5
	VMOV	R5, V19.S4

loop16:
	VLD1	(R0), [V0.B16]
	VUMIN	V16.B16, V0.B16, V1.B16
	VCMEQ	V16.B16, V1.B16, V1.B16 // 0xFF where byte >= ' '+1
	VAND	V19.B16, V1.B16, V1.B16
	VADDP	V1.B16, V1.B16, V2.B16
	VADDP	V2.B16, V2.B16, V2.B16
	VMOV	V2.D[0], R4
	CBNZ	R4, found16
	ADD	$16, R0, R0
	SUB	$16, R1, R1
	CMP	$16, R1
	BHS	loop16

tail:
	CBZ	R1, notfound

tailloop:
	MOVBU.P	1(R0), R5
	CMP	$32, R5
	BHI	tailfound
	SUBS	$1, R1, R1
	BNE	tailloop

notfound:
	MOVD	$-1, R4
	MOVD	R4, ret+24(FP)
	RET

found16:
	RBIT	R4, R4
	CLZ	R4, R4
	LSR	$1, R4, R4
	SUB	R3, R0, R6
	ADD	R6, R4, R4
	MOVD	R4, ret+24(FP)
	RET

tailfound:
	SUB	$1, R0, R0
	SUB	R3, R0, R4
	MOVD	R4, ret+24(FP)
	RET
