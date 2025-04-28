// +build sse
// +build amd64

#include "textflag.h"

// func mulAsm(a, b []float32)
TEXT ·mulAsm(SB), NOSPLIT, $0
	MOVQ a_data+0(FP), SI
	MOVQ b_data+24(FP), DI // use destination index register for this

	MOVQ a_len+8(FP), AX  // len(a) into AX

	// check if there are at least 16 elements
	SUBQ $16, AX
	JL   remainder

loop:
	// a[0]
	MOVAPS (SI), X0
	MOVAPS (DI), X1
	MULPS  X0, X1
	MOVAPS X1, (SI)

	MOVAPS 16(SI), X2
	MOVAPS 16(DI), X3
	MULPS  X2, X3
	MOVAPS X3, 16(SI)

	MOVAPS 32(SI), X4
	MOVAPS 32(DI), X5
	MULPS  X4, X5
	MOVAPS X5, 32(SI)

	MOVAPS 48(SI), X6
	MOVAPS 48(DI), X7
	MULPS  X6, X7
	MOVAPS X7, 48(SI)

	// update pointers. 4 registers, 4 elements at once, each element is 4 bytes
	ADDQ $64, SI
	ADDQ $64, DI

	// len(a) is now 4*4 elements less
	SUBQ $16, AX
	JGE  loop

remainder:
	ADDQ $16, AX
	JE   done

remainderloop:
	MOVSS (SI), X0
	MOVSS (DI), X1
	MULSS X0, X1
	MOVSS X1, (SI)

	// update pointer to the top of the data
	ADDQ $4, SI
	ADDQ $4, DI

	DECQ AX
	JNE  remainderloop

done:
	RET

