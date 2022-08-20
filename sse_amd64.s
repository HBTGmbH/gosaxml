#include "textflag.h"

DATA ·oab<>+0(SB)/8, $0x3C3C3C3C3C3C3C3C
DATA ·oab<>+8(SB)/8, $0x3C3C3C3C3C3C3C3C
DATA ·oab<>+16(SB)/8, $0x3C3C3C3C3C3C3C3C
DATA ·oab<>+24(SB)/8, $0x3C3C3C3C3C3C3C3C
GLOBL ·oab<>(SB), NOPTR+RODATA, $32

DATA ·spc<>+0(SB)/8, $0x2020202020202020
DATA ·spc<>+8(SB)/8, $0x2020202020202020
DATA ·spc<>+16(SB)/8, $0x2020202020202020
DATA ·spc<>+24(SB)/8, $0x2020202020202020
GLOBL ·spc<>(SB), NOPTR+RODATA, $32

DATA ·fifteen<>+0(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA ·fifteen<>+8(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA ·fifteen<>+16(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA ·fifteen<>+24(SB)/8, $0x0F0F0F0F0F0F0F0F
GLOBL ·fifteen<>(SB), NOPTR+RODATA, $32

DATA ·lo_nibbles<>+0(SB)/8, $0x0000000000000010
DATA ·lo_nibbles<>+8(SB)/8, $0x0804820000412000
DATA ·lo_nibbles<>+16(SB)/8, $0x0000000000000010
DATA ·lo_nibbles<>+24(SB)/8, $0x0804820000412000
GLOBL ·lo_nibbles<>(SB), NOPTR+RODATA, $32

DATA ·hi_nibbles<>+0(SB)/8, $0x00000000071800E0
DATA ·hi_nibbles<>+8(SB)/8, $0x0000000000000000
DATA ·hi_nibbles<>+16(SB)/8, $0x00000000071800E0
DATA ·hi_nibbles<>+24(SB)/8, $0x0000000000000000
GLOBL ·hi_nibbles<>(SB), NOPTR+RODATA, $32

TEXT ·openAngleBracket16(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    MOVOU (DI), X0
    PCMPEQB ·oab<>(SB), X0
    PMOVMSKB X0, AX
    TZCNTW AX, AX
    MOVB AX, ret+24(FP)
    RET

TEXT ·openAngleBracket32(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    VMOVDQU (DI), Y0
    VPCMPEQB ·oab<>(SB), Y0, Y0
    VPMOVMSKB Y0, AX
    TZCNTL AX, AX
    MOVB AX, ret+24(FP)
    VZEROUPPER // <- https://i.stack.imgur.com/dGpbi.png
    RET

TEXT ·onlySpaces16(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    MOVOU (DI), X0
    MOVOA X0, X1
    PCMPGTB ·spc<>(SB), X0
    PXOR X2, X2
    PCMPGTB X1, X2
    POR X2, X0
    PMOVMSKB X0, AX
    TZCNTW AX, AX
    MOVB AX, ret+24(FP)
    RET

TEXT ·onlySpaces32(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    VMOVDQU (DI), Y0
    VPCMPGTB ·spc<>(SB), Y0, Y1
    VPXOR X2, X2, X2
    VPCMPGTB Y0, Y2, Y0
    VPOR Y0, Y1, Y0
    VPMOVMSKB Y0, AX
    TZCNTL AX, AX
    MOVB AX, ret+24(FP)
    VZEROUPPER // <- https://i.stack.imgur.com/dGpbi.png
    RET

TEXT ·seperator32(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    VMOVDQU (DI), Y0

    // http://0x80.pl/articles/simd-byte-lookup.html#special-case-1-small-sets
    VPSRLW $4, Y0, Y1
    VMOVDQA ·fifteen<>(SB), Y2
    VPAND Y2, Y0, Y0
    VMOVDQA ·lo_nibbles<>(SB), Y3
    VPSHUFB Y0, Y3, Y0
    VPAND Y2, Y1, Y1
    VMOVDQA ·hi_nibbles<>(SB), Y2
    VPSHUFB Y1, Y2, Y1
    VPAND Y0, Y1, Y0

    // convert non-zero elements to 0xFF
    VPXOR X1, X1, X1 // <- generate all zeroes
    VPCMPEQB Y1, Y0, Y0 // <- convert all 0x00 to 0xFF and everything else to 0x00
    VPCMPEQD Y1, Y1, Y1 // <- generate all ones
    VPXOR Y0, Y1, Y0 // <- flip bits in xmm0

    // Extract index of first 1 bit
    VPMOVMSKB Y0, AX
    TZCNTL AX, AX

    // return
    MOVB AX, ret+24(FP)
    VZEROUPPER // <- https://i.stack.imgur.com/dGpbi.png
    RET
