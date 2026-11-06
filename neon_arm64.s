#include "textflag.h"

// NOTE: this file must assemble with the oldest Go toolchain the module
// supports (see go.mod). Assemblers before Go 1.27 know neither SHRN nor CMHI,
// so those two instructions are emitted as raw instruction words through the
// macros below. Each macro is bound to fixed registers; its comment gives the
// Go 1.27 spelling it can be replaced with once go.mod requires Go 1.27.

// SHRN $4 on the 16-bit lanes of a 0x00/0xFF byte mask keeps one nibble of
// every byte, condensing the mask into a 64-bit syndrome with four bits per
// input byte; the index of the first set lane is then ctz(syndrome)>>2. This
// is the NEON stand-in for x86's PMOVMSKB.
#define SHRN_V1 WORD $0x0F0C8421 // VSHRN $4, V1.H8, V1.B8
#define SHRN_V2 WORD $0x0F0C8442 // VSHRN $4, V2.H8, V2.B8
#define SHRN_V3 WORD $0x0F0C8463 // VSHRN $4, V3.H8, V3.B8

// CMHI against V17 (all lanes ' '): 0xFF where the input byte is > ' '.
#define CMHI_V1_V0 WORD $0x6E313401 // VCMHI V17.B16, V0.B16, V1.B16
#define CMHI_V2_V0 WORD $0x6E313402 // VCMHI V17.B16, V0.B16, V2.B16
#define CMHI_V3_V1 WORD $0x6E313423 // VCMHI V17.B16, V1.B16, V3.B16

// Membership table for the set of bytes that terminate an XML name:
//
//     '\t' 09, '\n' 0A, '\r' 0D, ' ' 20, '/' 2F, ':' 3A, '=' 3D, '>' 3E, '?' 3F
//
// All of them are below 0x40, so a single four-register TBL over this 64-byte
// table maps every input byte straight to 0xFF (separator) or 0x00. TBL yields
// 0 for indices >= 64, so all other bytes, including UTF-8 continuation bytes
// inside names, never match.
DATA ·sepTable<>+0(SB)/8, $0x0000000000000000  // 00..07
DATA ·sepTable<>+8(SB)/8, $0x0000FF0000FFFF00  // 08..0F: 09 0A 0D
DATA ·sepTable<>+16(SB)/8, $0x0000000000000000 // 10..17
DATA ·sepTable<>+24(SB)/8, $0x0000000000000000 // 18..1F
DATA ·sepTable<>+32(SB)/8, $0x00000000000000FF // 20..27: 20
DATA ·sepTable<>+40(SB)/8, $0xFF00000000000000 // 28..2F: 2F
DATA ·sepTable<>+48(SB)/8, $0x0000000000000000 // 30..37
DATA ·sepTable<>+56(SB)/8, $0xFFFFFF0000FF0000 // 38..3F: 3A 3D 3E 3F
GLOBL ·sepTable<>(SB), NOPTR+RODATA, $64

// Both scanners share the same contract and shape:
//
//   - They scan buf[from:] and return an index relative to the start of buf,
//     or -1.
//   - They require len(buf) >= 16 and that buf[:from] holds no match (the
//     caller's scalar head scan has already established that). This lets the
//     final, partial block be handled by reloading the last 16 bytes of buf:
//     whatever part of that window lies before the cursor is known not to
//     match, so the first hit in it is the right answer. No scalar tail loop
//     is needed.
//   - The syndrome is computed inside the loop (it doubles as the loop's exit
//     test), so that once the usually mispredicted loop exit resolves, only
//     RBIT/CLZ separate it from the result.
//
// Register use: R0 base, R1 end, R2 cursor, R3 start of the last 16-byte
// window, R4 syndrome.

// func indexSeparatorNEON(buf []byte, from int) int
TEXT ·indexSeparatorNEON(SB), NOSPLIT, $0-40
	MOVD	buf_base+0(FP), R0
	MOVD	buf_len+8(FP), R1
	MOVD	from+24(FP), R2
	MOVD	$·sepTable<>(SB), R5
	VLD1	(R5), [V20.B16, V21.B16, V22.B16, V23.B16]
	ADD	R0, R2, R2
	ADD	R0, R1, R1
	SUB	$16, R1, R3

	CMP	R3, R2
	BHI	last

loop16:
	VLD1.P	16(R2), [V0.B16]
	VTBL	V0.B16, [V20.B16, V21.B16, V22.B16, V23.B16], V1.B16
	SHRN_V1
	VMOV	V1.D[0], R4
	CBNZ	R4, found16
	CMP	R3, R2
	BLS	loop16

last:
	CMP	R1, R2
	BEQ	notfound
	VLD1	(R3), [V0.B16]
	VTBL	V0.B16, [V20.B16, V21.B16, V22.B16, V23.B16], V1.B16
	SHRN_V1
	VMOV	V1.D[0], R4
	CBZ	R4, notfound
	ADD	$16, R3, R2

found16: // R2 = end of the 16-byte window, R4 = its syndrome
	SUB	$16, R2, R2
	RBIT	R4, R4
	CLZ	R4, R4
	SUB	R0, R2, R2
	ADD	R4>>2, R2, R4
	MOVD	R4, ret+32(FP)
	RET

notfound:
	MOVD	$-1, R4
	MOVD	R4, ret+32(FP)
	RET

// func indexNonSpaceNEON(buf []byte, from int) int
//
// Finds the first byte greater than ' ' (i.e. the first byte that is not XML
// whitespace). Whitespace runs can be long, so the main loop consumes 32 bytes
// per iteration and only tests whether either half has a hit; the syndrome of
// the half that does is computed after the loop. At most one 16-byte step and
// the overlapping final window follow.
TEXT ·indexNonSpaceNEON(SB), NOSPLIT, $0-40
	MOVD	buf_base+0(FP), R0
	MOVD	buf_len+8(FP), R1
	MOVD	from+24(FP), R2
	VMOVI	$32, V17.B16
	ADD	R0, R2, R2
	ADD	R0, R1, R1
	SUB	$16, R1, R3
	SUB	$32, R1, R6             // R6 = start of the last 32-byte block

	CMP	R6, R2
	BHI	step16

loop32:
	VLD1.P	32(R2), [V0.B16, V1.B16]
	CMHI_V2_V0
	CMHI_V3_V1
	VORR	V2.B16, V3.B16, V4.B16
	VADDP	V4.D2, V4.D2, V4.D2     // non-zero iff any lane of either half is set
	VMOV	V4.D[0], R4
	CBNZ	R4, found32
	CMP	R6, R2
	BLS	loop32

step16:
	CMP	R3, R2
	BHI	last
	VLD1.P	16(R2), [V0.B16]
	CMHI_V1_V0
	SHRN_V1
	VMOV	V1.D[0], R4
	CBNZ	R4, found16

last:
	CMP	R1, R2
	BEQ	notfound
	VLD1	(R3), [V0.B16]
	CMHI_V1_V0
	SHRN_V1
	VMOV	V1.D[0], R4
	CBZ	R4, notfound
	ADD	$16, R3, R2

found16: // R2 = end of the 16-byte window, R4 = its syndrome
	SUB	$16, R2, R2

found: // R2 = start of the 16-byte window, R4 = its syndrome
	RBIT	R4, R4
	CLZ	R4, R4
	SUB	R0, R2, R2
	ADD	R4>>2, R2, R4
	MOVD	R4, ret+32(FP)
	RET

found32: // R2 = end of the 32-byte block, V2/V3 = lane masks of its halves
	SUB	$32, R2, R2
	SHRN_V2
	VMOV	V2.D[0], R4
	CBNZ	R4, found
	ADD	$16, R2, R2
	SHRN_V3
	VMOV	V3.D[0], R4
	B	found

notfound:
	MOVD	$-1, R4
	MOVD	R4, ret+32(FP)
	RET
