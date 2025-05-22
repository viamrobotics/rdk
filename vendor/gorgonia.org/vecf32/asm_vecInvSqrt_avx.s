// +build avx
// +build amd64
// +build !fastmath

/*
InvSqrt is a function that inverse square roots (1/√x) each element in a []float32

Because of the way VBROADCASTSS works, we first backup the first element of the slice
into a register, BX. Meanwhile, we replace the first element with a constant 1.0. 
This is done so that we can broadcast the constant into the Y1 register. After 1.0 has been 
broadcasted into Y1, we move the value back into the top of the slice. 

The following is then performed:
	Y0 = Sqrt(a[i:i+8])
	Y0 = Y1/Y0
And the standard looping thing happens
*/
#include "textflag.h"

#define one 0x3f800000

// func InvSqrt(a []float32)
TEXT ·InvSqrt(SB), NOSPLIT, $0
	MOVQ a_data+0(FP), SI
	MOVQ SI, CX
	MOVQ a_len+8(FP), AX  // len(a) into AX - +8, because first 8 is pointer, second 8 is length, third 8 is cap

	// make sure that len(a) >= 1
	XORQ BX, BX
	CMPQ BX, AX
	JGE  done

	MOVL $one, DX

	SUBQ $8, AX
	JL   remainder

	// store the first element in BX
	// This is done so that we can move 1.0 into the first element of the slice
	// because AVX instruction vbroadcastss can only read from memory location not from registers
	MOVL (SI), BX

	// load 1.0 into the first element
	MOVL DX, (SI)

	// VBROADCASTSS (SI), Y1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x7d; BYTE $0x18; BYTE $0x0e // vbroadcastss (%rsi),%ymm1

	// now that we're done with the ghastly business of trying to broadcast 1.0 without using any extra memory...
	// we restore the first element
	MOVL BX, (SI)

loop:
	// a[0] to a[7]
	// VSQRTPS (SI), Y0
	// VDIVPS Y0, Y1, Y0
	// VMOVUPS Y0, (SI)
	BYTE $0xc5; BYTE $0xfc; BYTE $0x51; BYTE $0x06 // vsqrtpd (%rsi),%ymm0
	BYTE $0xc5; BYTE $0xf4; BYTE $0x5e; BYTE $0xc0 // vdivps %ymm0, %ymm1, %ymm0
	BYTE $0xc5; BYTE $0xfc; BYTE $0x11; BYTE $0x06 // vmovups %ymm0, (%rsi)

	ADDQ $32, SI
	SUBQ $8, AX
	JGE  loop

remainder:
	ADDQ $8, AX
	JE   done

remainder1:
	MOVQ   DX, X1
	MOVSS  (SI), X0
	SQRTSS X0, X0
	DIVSS  X0, X1
	MOVSS  X1, (SI)

	ADDQ $4, SI
	DECQ AX
	JNE  remainder1

done:
	RET
