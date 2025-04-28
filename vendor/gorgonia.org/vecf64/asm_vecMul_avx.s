// +build avx
// +build amd64

/*
Mul multiplies two []float64 with some SIMD optimizations using AVX.

Instead of doing this:
	for i := 0; i < len(a); i++ {
		a[i] *= b[i]
	}

Here, I use the term "pairs" to denote an element of `a` and and element of `b` that will be added together. 
a[i], b[i] is a pair.

Using AVX, we can simultaneously add 16 pairs at the same time, which will look something like this:
	for i := 0; i < len(a); i+=4{
		a[i:i+4] += b[i:i+4] // this code won't run.
	}

AVX registers are 256 bits, meaning we can put 4 float64s in there. 

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
		a[i:i+4*8] *= b[i:i+4*8]
		lenA -= 4
		i += 4 * 8 // 4 elements, 8 bytes each
		
		if lenA < 0{
			break
		}
	}

	remainder4head:
	lenA += 4
	if lenA == 0 {
		return
	}

	remainder2:
	for {
		a[i:i+2*8] *= b[i:i+2*8]
		lenA -=2
		i += 2 * 8  // 2 elements, 8 bytes each
		
		if lenA < 0{
			break
		}
	}

	remainder1head:
	lenA += 2
	if lenA == 0 {
		return
	}

	remainder1:
	for {
		a[i] *= b[i]
		i+=8 // each element is 8 bytes
		lenA--
	}

	return

*/
#include "textflag.h"

// func mulAsm(a, b []float64)
TEXT ·mulAsm(SB), NOSPLIT, $0
	MOVQ a_data+0(FP), SI
	MOVQ b_data+24(FP), DI // use detination index register for this

	MOVQ a_len+8(FP), AX  // len(a) into AX

	// each ymm register can take up to 4 float64s.
	SUBQ $4, AX
	JL   remainder

loop:
	// a[0] to a[3]
	// VMOVUPD 0(SI), Y0
	// VMOVUPD 0(DI), Y1
	// VMULPD Y0, Y1, Y0
	// VMOVUPD  Y0, 0(SI)
	BYTE $0xc5; BYTE $0xfd; BYTE $0x10; BYTE $0x06 // vmovupd (%rsi),%ymm0
	BYTE $0xc5; BYTE $0xfd; BYTE $0x10; BYTE $0x0f // vmovupd (%rdi),%ymm1
	BYTE $0xc5; BYTE $0xf5; BYTE $0x59; BYTE $0xc0 // vmulpd %ymm0,%ymm1,%ymm0
	BYTE $0xc5; BYTE $0xfd; BYTE $0x11; BYTE $0x06 // vmovupd %ymm0,(%rsi)

	ADDQ $32, SI
	ADDQ $32, DI
	SUBQ $4, AX
	JGE  loop

remainder:
	ADDQ $4, AX
	JE   done

	SUBQ $2, AX
	JL   remainder1head

remainder2:
	// VMOVUPD (SI), X0
	// VMOVUPD (DI), X1
	// VMULPD X0, X1, X0
	// VMOVUPD X0, (SI)
	BYTE $0xc5; BYTE $0xf9; BYTE $0x10; BYTE $0x06 // vmovupd (%rsi),%xmm0
	BYTE $0xc5; BYTE $0xf9; BYTE $0x10; BYTE $0x0f // vmovupd (%rdi),%xmm1
	BYTE $0xc5; BYTE $0xf1; BYTE $0x59; BYTE $0xc0 // vmulpd %xmm0,%xmm1,%xmm0
	BYTE $0xc5; BYTE $0xf9; BYTE $0x11; BYTE $0x06 // vmovupd %xmm0,(%rsi)

	ADDQ $16, SI
	ADDQ $16, DI
	SUBQ $2, AX
	JGE  remainder2

remainder1head:
	ADDQ $2, AX
	JE   done

remainder1:
	// copy into the appropriate registers
	// VMOVSD	(SI), X0
	// VMOVSD	(DI), X1
	// VADDSD	X0, X1, X0
	// VMOVSD	X0, (SI)
	BYTE $0xc5; BYTE $0xfb; BYTE $0x10; BYTE $0x06 // vmovsd (%rsi),%xmm0
	BYTE $0xc5; BYTE $0xfb; BYTE $0x10; BYTE $0x0f // vmovsd (%rdi),%xmm1
	BYTE $0xc5; BYTE $0xf3; BYTE $0x59; BYTE $0xc0 // vmulsd %xmm0,%xmm1,%xmm0
	BYTE $0xc5; BYTE $0xfb; BYTE $0x11; BYTE $0x06 // vmovsd %xmm0,(%rsi)

	// update pointer to the top of the data
	ADDQ $8, SI
	ADDQ $8, DI

	DECQ AX
	JNE  remainder1

done:
	RET
