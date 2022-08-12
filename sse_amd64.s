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

TEXT ·openAngleBracket16(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    MOVOU (DI), X0
    PCMPEQB ·oab<>(SB), X0
    PMOVMSKB X0, AX
    TZCNTW AX, AX
    MOVW AX, ret+24(FP)
    RET

TEXT ·openAngleBracket32(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    VMOVDQU (DI), Y0
    VPCMPEQB ·oab<>(SB), Y0, Y0
    VPMOVMSKB Y0, AX
    TZCNTL AX, AX
    MOVQ AX, ret+24(FP)
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
    MOVW AX, ret+24(FP)
    RET

TEXT ·onlySpaces32(SB),NOSPLIT, $0
    MOVQ arg+0(FP), DI
    VMOVDQU (DI), Y0
    VMOVDQA Y0, Y1
    VPCMPGTB ·spc<>(SB), Y0, Y0
    VPXOR Y2, Y2, Y2
    VPCMPGTB Y1, Y2, Y2
    VPOR Y2, Y0, Y0
    VPMOVMSKB Y0, AX
    MOVL AX, ret+24(FP)
    VZEROUPPER // <- https://i.stack.imgur.com/dGpbi.png
    RET
