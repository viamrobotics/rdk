// +build avx
// +build amd64

/*
Mul multiplies two []float32 with some SIMD optimizations using AVX.

Instead of doing this:
	for i := 0; i < len(a); i++ {
	    a[i] *= b[i]
	}

Here, I use the term "pairs" to denote an element of `a` and and element of `b` that will be added together. 
a[i], b[i] is a pair.

Using AVX, we can simultaneously add 8 pairs at the same time, which will look something like this:
	for i := 0; i < len(a); i+=8{
	    a[i:i+8] *= b[i:i+8] // this code won't run.
	}

AVX registers are 256 bits, meaning we can put 8 float32s in there. 

These are the registers I use to store the relevant information:
	SI - Used to store the top element of slice A (index 0). This register is incremented every loop
	DI - used to store the top element of slice B. Incremented every loop
	AX - len(a) is stored in here. AX is also used as the "working" count of the length that is decremented.
	Y0, Y1 - YMM registers. 
	X0, X1 - XMM registers.

This pseudocode best explains the rather simple assembly:

	lenA := len(a)
	i := 0

	loop:
	for {
		a[i:i+8*4] *= b[i:i+8*4]
		lenA -= 8
		i += 8 * 4 // 8 elements, 4 bytes each
		
		if lenA < 0{
			break
		}
	}

	remainder4head:
	lenA += 8
	if lenA == 0 {
		return
	}

	remainder4:
	for {
		a[i:i+4*4] *= b[i:i+4*4]
		lenA -=4
		i += 4 * 4  // 4 elements, 4 bytes each
		
		if lenA < 0{
			break
		}
	}

	remainder1head:
	lenA += 4
	if lenA == 0 {
		return
	}

	remainder1:
	for {
		a[i] *= b[i]
		i+=4 // each element is 4 bytes
		lenA--
	}

	return

*/
#include "textflag.h"

// func mulAsm(a, b []float32)
TEXT ·mulAsm(SB), NOSPLIT, $0
	MOVQ a_data+0(FP), SI
	MOVQ b_data+24(FP), DI // use detination index register for this

	MOVQ a_len+8(FP), AX  // len(a) into AX

	// each ymm register can take up to 8 float32s.
	SUBQ $8, AX
	JL   remainder

loop:
	// a[0] to a[7]
	// VMOVUPS 0(SI), Y0
	// VMOVUPS 0(DI), Y1
	// VMULPS Y0, Y1, Y0
	// VMOVUPS  Y0, 0(SI)
	BYTE $0xc5; BYTE $0xfc; BYTE $0x10; BYTE $0x06 // vmovups (%rsi),%ymm0
	BYTE $0xc5; BYTE $0xfc; BYTE $0x10; BYTE $0x0f // vmovups (%rdi),%ymm1
	BYTE $0xc5; BYTE $0xf4; BYTE $0x59; BYTE $0xc0 // vmulps %ymm0,%ymm1,%ymm0
	BYTE $0xc5; BYTE $0xfc; BYTE $0x11; BYTE $0x06 // vmovups %ymm0,(%rsi)

	ADDQ $32, SI
	ADDQ $32, DI
	SUBQ $8, AX
	JGE  loop

remainder:
	ADDQ $8, AX
	JE   done

	SUBQ $4, AX
	JL   remainder1head

remainder4:
	// VMOVUPS (SI), X0
	// VMOVUPS (DI), X1
	// VMULPS  X0, X1, X0
	// VMOVUPS X0 (SI)
	BYTE $0xc5; BYTE $0xf8; BYTE $0x10; BYTE $0x06 // vmovups (%rsi),%xmm0
	BYTE $0xc5; BYTE $0xf8; BYTE $0x10; BYTE $0x0f // vmovups (%rdi),%xmm1
	BYTE $0xc5; BYTE $0xf0; BYTE $0x59; BYTE $0xc0 // vmulps %xmm0,%xmm1,%xmm0
	BYTE $0xc5; BYTE $0xf8; BYTE $0x11; BYTE $0x06 // vmovups %xmm0,(%rsi)

	ADDQ $16, SI
	ADDQ $16, DI
	SUBQ $4, AX
	JGE  remainder4

remainder1head:
	ADDQ $4, AX
	JE   done

remainder1:
	// copy into the appropriate registers
	// VMOVSS	(SI), X0
	// VMOVSS	(DI), X1
	// VMULSS	X0, X1, X0
	// VMOVSS	X0, (SI)
	BYTE $0xc5; BYTE $0xfa; BYTE $0x10; BYTE $0x06 // vmovss (%rsi),%xmm0
	BYTE $0xc5; BYTE $0xfa; BYTE $0x10; BYTE $0x0f // vmovss (%rdi),%xmm1
	BYTE $0xc5; BYTE $0xf2; BYTE $0x59; BYTE $0xc0 // vmulss %xmm0,%xmm1,%xmm0
	BYTE $0xc5; BYTE $0xfa; BYTE $0x11; BYTE $0x06 // vmovss %xmm0,(%rsi)

	// update pointer to the top of the data
	ADDQ $4, SI
	ADDQ $4, DI

	DECQ AX
	JNE  remainder1

done:
	RET
