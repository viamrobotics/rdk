#include "textflag.h"

DATA sat_absMask<>+0(SB)/8, $0x7FFFFFFFFFFFFFFF
GLOBL sat_absMask<>(SB), RODATA|NOPTR, $8

// func obbSATMaxGap(input *[27]float64) float64
//
// Input layout (27 float64s at pointer SI):
//   0(SI)..64(SI)    rmA[0..8]   (row-major 3×3)
//   72(SI)..136(SI)  rmB[0..8]   (row-major 3×3)
//   144(SI)..160(SI) halfSizeA[0..2]
//   168(SI)..184(SI) halfSizeB[0..2]
//   192(SI)..208(SI) centerDist[0..2]
//
// Stack frame layout (144 bytes):
//   0(SP)..64(SP)    R[0..8]     (relative rotation)
//   72(SP)..136(SP)  absR[0..8]  (|R| + epsilon)
//
// Register plan:
//   SI   = input pointer (preserved)
//   X9   = T[0]
//   X10  = T[1]
//   X11  = T[2]
//   X13  = running max (best)
//   X14  = epsilon constant (1e-10)
//   X15  = abs mask (0x7FFFFFFFFFFFFFFF)
//   X0-X8, X12 = temporaries

TEXT ·obbSATMaxGap(SB), NOSPLIT, $144-16
	MOVQ input+0(FP), SI

	// --- Setup constants ---
	// Abs mask: 0x7FFFFFFFFFFFFFFF (clears sign bit for float64 abs)
	MOVQ sat_absMask<>(SB), X15

	// Epsilon = 1e-10
	MOVQ $0x3DDB7CDFD9D7BDBB, AX
	MOVQ AX, X14

	// best = -Inf
	MOVQ $0xFFF0000000000000, AX
	MOVQ AX, X13

	// --- Compute T[i] = rmA.Row(i) · centerDist ---
	// Load centerDist into X5, X6, X7 (reused across T computations)
	MOVSD 192(SI), X5
	MOVSD 200(SI), X6
	MOVSD 208(SI), X7

	// T[0] = rmA[0]*cd[0] + rmA[1]*cd[1] + rmA[2]*cd[2]
	MOVSD 0(SI), X0
	MULSD X5, X0
	MOVSD 8(SI), X1
	MULSD X6, X1
	ADDSD X1, X0
	MOVSD 16(SI), X1
	MULSD X7, X1
	ADDSD X1, X0
	MOVAPD X0, X9      // X9 = T[0]

	// T[1] = rmA[3]*cd[0] + rmA[4]*cd[1] + rmA[5]*cd[2]
	MOVSD 24(SI), X0
	MULSD X5, X0
	MOVSD 32(SI), X1
	MULSD X6, X1
	ADDSD X1, X0
	MOVSD 40(SI), X1
	MULSD X7, X1
	ADDSD X1, X0
	MOVAPD X0, X10     // X10 = T[1]

	// T[2] = rmA[6]*cd[0] + rmA[7]*cd[1] + rmA[8]*cd[2]
	MOVSD 48(SI), X0
	MULSD X5, X0
	MOVSD 56(SI), X1
	MULSD X6, X1
	ADDSD X1, X0
	MOVSD 64(SI), X1
	MULSD X7, X1
	ADDSD X1, X0
	MOVAPD X0, X11     // X11 = T[2]

	// --- Compute R[i][j] and absR[i][j], store on stack ---
	// R[i][j] = rmA.Row(i) · rmB.Row(j)
	// absR[i][j] = |R[i][j]| + epsilon

	// Load rmA.Row(0) into X5, X6, X7
	MOVSD 0(SI), X5
	MOVSD 8(SI), X6
	MOVSD 16(SI), X7

	// R[0][0] = rmA.Row(0) · rmB.Row(0)
	MOVSD X5, X0
	MULSD 72(SI), X0
	MOVSD X6, X1
	MULSD 80(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 88(SI), X1
	ADDSD X1, X0
	MOVSD X0, 0(SP)        // R[0][0]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 72(SP)      // absR[0][0]

	// R[0][1] = rmA.Row(0) · rmB.Row(1)
	MOVSD X5, X0
	MULSD 96(SI), X0
	MOVSD X6, X1
	MULSD 104(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 112(SI), X1
	ADDSD X1, X0
	MOVSD X0, 8(SP)        // R[0][1]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 80(SP)      // absR[0][1]

	// R[0][2] = rmA.Row(0) · rmB.Row(2)
	MOVSD X5, X0
	MULSD 120(SI), X0
	MOVSD X6, X1
	MULSD 128(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 136(SI), X1
	ADDSD X1, X0
	MOVSD X0, 16(SP)       // R[0][2]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 88(SP)      // absR[0][2]

	// Load rmA.Row(1) into X5, X6, X7
	MOVSD 24(SI), X5
	MOVSD 32(SI), X6
	MOVSD 40(SI), X7

	// R[1][0]
	MOVSD X5, X0
	MULSD 72(SI), X0
	MOVSD X6, X1
	MULSD 80(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 88(SI), X1
	ADDSD X1, X0
	MOVSD X0, 24(SP)       // R[1][0]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 96(SP)      // absR[1][0]

	// R[1][1]
	MOVSD X5, X0
	MULSD 96(SI), X0
	MOVSD X6, X1
	MULSD 104(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 112(SI), X1
	ADDSD X1, X0
	MOVSD X0, 32(SP)       // R[1][1]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 104(SP)     // absR[1][1]

	// R[1][2]
	MOVSD X5, X0
	MULSD 120(SI), X0
	MOVSD X6, X1
	MULSD 128(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 136(SI), X1
	ADDSD X1, X0
	MOVSD X0, 40(SP)       // R[1][2]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 112(SP)     // absR[1][2]

	// Load rmA.Row(2) into X5, X6, X7
	MOVSD 48(SI), X5
	MOVSD 56(SI), X6
	MOVSD 64(SI), X7

	// R[2][0]
	MOVSD X5, X0
	MULSD 72(SI), X0
	MOVSD X6, X1
	MULSD 80(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 88(SI), X1
	ADDSD X1, X0
	MOVSD X0, 48(SP)       // R[2][0]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 120(SP)     // absR[2][0]

	// R[2][1]
	MOVSD X5, X0
	MULSD 96(SI), X0
	MOVSD X6, X1
	MULSD 104(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 112(SI), X1
	ADDSD X1, X0
	MOVSD X0, 56(SP)       // R[2][1]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 128(SP)     // absR[2][1]

	// R[2][2]
	MOVSD X5, X0
	MULSD 120(SI), X0
	MOVSD X6, X1
	MULSD 128(SI), X1
	ADDSD X1, X0
	MOVSD X7, X1
	MULSD 136(SI), X1
	ADDSD X1, X0
	MOVSD X0, 64(SP)       // R[2][2]
	MOVAPD X0, X1
	ANDPD  X15, X1
	ADDSD  X14, X1
	MOVSD  X1, 136(SP)     // absR[2][2]

	// ============================================================
	// Face axis A0: gap = |T[0]| - hA[0] - (hB[0]*ar00 + hB[1]*ar01 + hB[2]*ar02)
	// ============================================================
	MOVAPD X9, X0          // T[0]
	ANDPD  X15, X0         // |T[0]|
	MOVSD  144(SI), X1     // hA[0]
	SUBSD  X1, X0          // |T[0]| - hA[0]
	MOVSD  168(SI), X1     // hB[0]
	MULSD  72(SP), X1      // hB[0]*ar00
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  80(SP), X1      // hB[1]*ar01
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  88(SP), X1      // hB[2]*ar02
	SUBSD  X1, X0
	MAXSD  X0, X13         // best = max(best, gap)

	// Face axis A1: gap = |T[1]| - hA[1] - (hB[0]*ar10 + hB[1]*ar11 + hB[2]*ar12)
	MOVAPD X10, X0
	ANDPD  X15, X0
	MOVSD  152(SI), X1
	SUBSD  X1, X0
	MOVSD  168(SI), X1
	MULSD  96(SP), X1
	SUBSD  X1, X0
	MOVSD  176(SI), X1
	MULSD  104(SP), X1
	SUBSD  X1, X0
	MOVSD  184(SI), X1
	MULSD  112(SP), X1
	SUBSD  X1, X0
	MAXSD  X0, X13

	// Face axis A2: gap = |T[2]| - hA[2] - (hB[0]*ar20 + hB[1]*ar21 + hB[2]*ar22)
	MOVAPD X11, X0
	ANDPD  X15, X0
	MOVSD  160(SI), X1
	SUBSD  X1, X0
	MOVSD  168(SI), X1
	MULSD  120(SP), X1
	SUBSD  X1, X0
	MOVSD  176(SI), X1
	MULSD  128(SP), X1
	SUBSD  X1, X0
	MOVSD  184(SI), X1
	MULSD  136(SP), X1
	SUBSD  X1, X0
	MAXSD  X0, X13

	// ============================================================
	// Face axis B0: gap = |T[0]*R00 + T[1]*R10 + T[2]*R20| - hB[0]
	//                     - (hA[0]*ar00 + hA[1]*ar10 + hA[2]*ar20)
	// ============================================================
	MOVAPD X9, X0
	MULSD  0(SP), X0       // T[0]*R00
	MOVAPD X10, X1
	MULSD  24(SP), X1      // T[1]*R10
	ADDSD  X1, X0
	MOVAPD X11, X1
	MULSD  48(SP), X1      // T[2]*R20
	ADDSD  X1, X0
	ANDPD  X15, X0         // abs
	MOVSD  168(SI), X1     // hB[0]
	SUBSD  X1, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  72(SP), X1      // hA[0]*ar00
	SUBSD  X1, X0
	MOVSD  152(SI), X1     // hA[1]
	MULSD  96(SP), X1      // hA[1]*ar10
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  120(SP), X1     // hA[2]*ar20
	SUBSD  X1, X0
	MAXSD  X0, X13

	// Face axis B1: gap = |T[0]*R01 + T[1]*R11 + T[2]*R21| - hB[1]
	//                     - (hA[0]*ar01 + hA[1]*ar11 + hA[2]*ar21)
	MOVAPD X9, X0
	MULSD  8(SP), X0
	MOVAPD X10, X1
	MULSD  32(SP), X1
	ADDSD  X1, X0
	MOVAPD X11, X1
	MULSD  56(SP), X1
	ADDSD  X1, X0
	ANDPD  X15, X0
	MOVSD  176(SI), X1
	SUBSD  X1, X0
	MOVSD  144(SI), X1
	MULSD  80(SP), X1
	SUBSD  X1, X0
	MOVSD  152(SI), X1
	MULSD  104(SP), X1
	SUBSD  X1, X0
	MOVSD  160(SI), X1
	MULSD  128(SP), X1
	SUBSD  X1, X0
	MAXSD  X0, X13

	// Face axis B2: gap = |T[0]*R02 + T[1]*R12 + T[2]*R22| - hB[2]
	//                     - (hA[0]*ar02 + hA[1]*ar12 + hA[2]*ar22)
	MOVAPD X9, X0
	MULSD  16(SP), X0
	MOVAPD X10, X1
	MULSD  40(SP), X1
	ADDSD  X1, X0
	MOVAPD X11, X1
	MULSD  64(SP), X1
	ADDSD  X1, X0
	ANDPD  X15, X0
	MOVSD  184(SI), X1
	SUBSD  X1, X0
	MOVSD  144(SI), X1
	MULSD  88(SP), X1
	SUBSD  X1, X0
	MOVSD  152(SI), X1
	MULSD  112(SP), X1
	SUBSD  X1, X0
	MOVSD  160(SI), X1
	MULSD  136(SP), X1
	SUBSD  X1, X0
	MAXSD  X0, X13

	// ============================================================
	// Edge axes: a_i × b_j with normalization by sqrt(1 - R[i][j]^2)
	// Skip if 1 - R[i][j]^2 <= 1e-12 (degenerate / parallel)
	//
	// For each edge axis:
	//   raw = |proj_center| - extentA - extentB
	//   gap = raw / sqrt(1 - R[i][j]^2)
	// ============================================================

	// Load 1.0 into X12 (reused for all edge tests)
	MOVQ $0x3FF0000000000000, AX
	MOVQ AX, X12

	// Load degeneracy threshold 1e-12 into X8
	MOVQ $0x3D719799812DEA11, AX
	MOVQ AX, X8

	// --- Edge a0 × b0 ---
	// l2 = 1 - R00^2
	MOVSD 0(SP), X0        // R00
	MOVAPD X0, X1
	MULSD  X0, X1          // R00^2
	MOVAPD X12, X2         // 1.0
	SUBSD  X1, X2          // l2 = 1 - R00^2
	UCOMISD X8, X2         // compare l2 vs 1e-12
	JBE    skip_e00        // skip if l2 <= 1e-12
	// raw = |T[2]*R10 - T[1]*R20| - (hA[1]*ar20 + hA[2]*ar10) - (hB[1]*ar02 + hB[2]*ar01)
	MOVAPD X11, X0         // T[2]
	MULSD  24(SP), X0      // T[2]*R10
	MOVAPD X10, X1         // T[1]
	MULSD  48(SP), X1      // T[1]*R20
	SUBSD  X1, X0          // T[2]*R10 - T[1]*R20
	ANDPD  X15, X0         // abs
	MOVSD  152(SI), X1     // hA[1]
	MULSD  120(SP), X1     // hA[1]*ar20
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  96(SP), X1      // hA[2]*ar10
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  88(SP), X1      // hB[1]*ar02
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  80(SP), X1      // hB[2]*ar01
	SUBSD  X1, X0
	// gap = raw / sqrt(l2)
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e00:

	// --- Edge a0 × b1 ---
	// l2 = 1 - R01^2
	MOVSD 8(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e01
	// raw = |T[2]*R11 - T[1]*R21| - (hA[1]*ar21 + hA[2]*ar11) - (hB[0]*ar02 + hB[2]*ar00)
	MOVAPD X11, X0
	MULSD  32(SP), X0      // T[2]*R11
	MOVAPD X10, X1
	MULSD  56(SP), X1      // T[1]*R21
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  152(SI), X1     // hA[1]
	MULSD  128(SP), X1     // hA[1]*ar21
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  104(SP), X1     // hA[2]*ar11
	SUBSD  X1, X0
	MOVSD  168(SI), X1     // hB[0]
	MULSD  88(SP), X1      // hB[0]*ar02
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  72(SP), X1      // hB[2]*ar00
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e01:

	// --- Edge a0 × b2 ---
	// l2 = 1 - R02^2
	MOVSD 16(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e02
	// raw = |T[2]*R12 - T[1]*R22| - (hA[1]*ar22 + hA[2]*ar12) - (hB[0]*ar01 + hB[1]*ar00)
	MOVAPD X11, X0
	MULSD  40(SP), X0      // T[2]*R12
	MOVAPD X10, X1
	MULSD  64(SP), X1      // T[1]*R22
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  152(SI), X1     // hA[1]
	MULSD  136(SP), X1     // hA[1]*ar22
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  112(SP), X1     // hA[2]*ar12
	SUBSD  X1, X0
	MOVSD  168(SI), X1     // hB[0]
	MULSD  80(SP), X1      // hB[0]*ar01
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  72(SP), X1      // hB[1]*ar00
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e02:

	// --- Edge a1 × b0 ---
	// l2 = 1 - R10^2
	MOVSD 24(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e10
	// raw = |T[0]*R20 - T[2]*R00| - (hA[0]*ar20 + hA[2]*ar00) - (hB[1]*ar12 + hB[2]*ar11)
	MOVAPD X9, X0
	MULSD  48(SP), X0      // T[0]*R20
	MOVAPD X11, X1
	MULSD  0(SP), X1       // T[2]*R00
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  120(SP), X1     // hA[0]*ar20
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  72(SP), X1      // hA[2]*ar00
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  112(SP), X1     // hB[1]*ar12
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  104(SP), X1     // hB[2]*ar11
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e10:

	// --- Edge a1 × b1 ---
	// l2 = 1 - R11^2
	MOVSD 32(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e11
	// raw = |T[0]*R21 - T[2]*R01| - (hA[0]*ar21 + hA[2]*ar01) - (hB[0]*ar12 + hB[2]*ar10)
	MOVAPD X9, X0
	MULSD  56(SP), X0      // T[0]*R21
	MOVAPD X11, X1
	MULSD  8(SP), X1       // T[2]*R01
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  128(SP), X1     // hA[0]*ar21
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  80(SP), X1      // hA[2]*ar01
	SUBSD  X1, X0
	MOVSD  168(SI), X1     // hB[0]
	MULSD  112(SP), X1     // hB[0]*ar12
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  96(SP), X1      // hB[2]*ar10
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e11:

	// --- Edge a1 × b2 ---
	// l2 = 1 - R12^2
	MOVSD 40(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e12
	// raw = |T[0]*R22 - T[2]*R02| - (hA[0]*ar22 + hA[2]*ar02) - (hB[0]*ar11 + hB[1]*ar10)
	MOVAPD X9, X0
	MULSD  64(SP), X0      // T[0]*R22
	MOVAPD X11, X1
	MULSD  16(SP), X1      // T[2]*R02
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  136(SP), X1     // hA[0]*ar22
	SUBSD  X1, X0
	MOVSD  160(SI), X1     // hA[2]
	MULSD  88(SP), X1      // hA[2]*ar02
	SUBSD  X1, X0
	MOVSD  168(SI), X1     // hB[0]
	MULSD  104(SP), X1     // hB[0]*ar11
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  96(SP), X1      // hB[1]*ar10
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e12:

	// --- Edge a2 × b0 ---
	// l2 = 1 - R20^2
	MOVSD 48(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e20
	// raw = |T[1]*R00 - T[0]*R10| - (hA[0]*ar10 + hA[1]*ar00) - (hB[1]*ar22 + hB[2]*ar21)
	MOVAPD X10, X0
	MULSD  0(SP), X0       // T[1]*R00
	MOVAPD X9, X1
	MULSD  24(SP), X1      // T[0]*R10
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  96(SP), X1      // hA[0]*ar10
	SUBSD  X1, X0
	MOVSD  152(SI), X1     // hA[1]
	MULSD  72(SP), X1      // hA[1]*ar00
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  136(SP), X1     // hB[1]*ar22
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  128(SP), X1     // hB[2]*ar21
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e20:

	// --- Edge a2 × b1 ---
	// l2 = 1 - R21^2
	MOVSD 56(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e21
	// raw = |T[1]*R01 - T[0]*R11| - (hA[0]*ar11 + hA[1]*ar01) - (hB[0]*ar22 + hB[2]*ar20)
	MOVAPD X10, X0
	MULSD  8(SP), X0       // T[1]*R01
	MOVAPD X9, X1
	MULSD  32(SP), X1      // T[0]*R11
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  104(SP), X1     // hA[0]*ar11
	SUBSD  X1, X0
	MOVSD  152(SI), X1     // hA[1]
	MULSD  80(SP), X1      // hA[1]*ar01
	SUBSD  X1, X0
	MOVSD  168(SI), X1     // hB[0]
	MULSD  136(SP), X1     // hB[0]*ar22
	SUBSD  X1, X0
	MOVSD  184(SI), X1     // hB[2]
	MULSD  120(SP), X1     // hB[2]*ar20
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e21:

	// --- Edge a2 × b2 ---
	// l2 = 1 - R22^2
	MOVSD 64(SP), X0
	MOVAPD X0, X1
	MULSD  X0, X1
	MOVAPD X12, X2
	SUBSD  X1, X2
	UCOMISD X8, X2
	JBE    skip_e22
	// raw = |T[1]*R02 - T[0]*R12| - (hA[0]*ar12 + hA[1]*ar02) - (hB[0]*ar21 + hB[1]*ar20)
	MOVAPD X10, X0
	MULSD  16(SP), X0      // T[1]*R02
	MOVAPD X9, X1
	MULSD  40(SP), X1      // T[0]*R12
	SUBSD  X1, X0
	ANDPD  X15, X0
	MOVSD  144(SI), X1     // hA[0]
	MULSD  112(SP), X1     // hA[0]*ar12
	SUBSD  X1, X0
	MOVSD  152(SI), X1     // hA[1]
	MULSD  88(SP), X1      // hA[1]*ar02
	SUBSD  X1, X0
	MOVSD  168(SI), X1     // hB[0]
	MULSD  128(SP), X1     // hB[0]*ar21
	SUBSD  X1, X0
	MOVSD  176(SI), X1     // hB[1]
	MULSD  120(SP), X1     // hB[1]*ar20
	SUBSD  X1, X0
	SQRTSD X2, X3
	DIVSD  X3, X0
	MAXSD  X0, X13
skip_e22:

	// --- Return best ---
	MOVSD X13, ret+8(FP)
	RET
