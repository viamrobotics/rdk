package spatialmath

import "math"

// obbSATMaxGap computes the maximum separation gap across all 15 SAT axes
// for two oriented bounding boxes using Ericson's precomputed R-matrix formulation
// ("Real-Time Collision Detection" Ch. 4.4).
//
// Input layout (27 float64s, row-major):
//
//	[0..8]   rmA rotation matrix
//	[9..17]  rmB rotation matrix
//	[18..20] halfSizeA
//	[21..23] halfSizeB
//	[24..26] centerDist (cB - cA)
//
// Returns the maximum gap across all 15 axes:
//   - Positive: boxes are separated by at least this distance
//   - Negative: boxes overlap with this penetration depth
func obbSATMaxGap(input *[27]float64) float64 {
	const eps = 1e-10

	// Unpack rotation matrices.
	a0, a1, a2 := input[0], input[1], input[2]
	a3, a4, a5 := input[3], input[4], input[5]
	a6, a7, a8 := input[6], input[7], input[8]
	b0, b1, b2 := input[9], input[10], input[11]
	b3, b4, b5 := input[12], input[13], input[14]
	b6, b7, b8 := input[15], input[16], input[17]

	// Half sizes and center distance.
	hA0, hA1, hA2 := input[18], input[19], input[20]
	hB0, hB1, hB2 := input[21], input[22], input[23]
	cdx, cdy, cdz := input[24], input[25], input[26]

	// T[i] = rmA.Row(i) . centerDist — center distance in A's frame.
	t0 := a0*cdx + a1*cdy + a2*cdz
	t1 := a3*cdx + a4*cdy + a5*cdz
	t2 := a6*cdx + a7*cdy + a8*cdz

	// R[i][j] = rmA.Row(i) . rmB.Row(j) — relative rotation.
	r00 := a0*b0 + a1*b1 + a2*b2
	r01 := a0*b3 + a1*b4 + a2*b5
	r02 := a0*b6 + a1*b7 + a2*b8
	r10 := a3*b0 + a4*b1 + a5*b2
	r11 := a3*b3 + a4*b4 + a5*b5
	r12 := a3*b6 + a4*b7 + a5*b8
	r20 := a6*b0 + a7*b1 + a8*b2
	r21 := a6*b3 + a7*b4 + a8*b5
	r22 := a6*b6 + a7*b7 + a8*b8

	// absR[i][j] = |R[i][j]| + eps — epsilon prevents issues with near-parallel edges.
	ar00 := math.Abs(r00) + eps
	ar01 := math.Abs(r01) + eps
	ar02 := math.Abs(r02) + eps
	ar10 := math.Abs(r10) + eps
	ar11 := math.Abs(r11) + eps
	ar12 := math.Abs(r12) + eps
	ar20 := math.Abs(r20) + eps
	ar21 := math.Abs(r21) + eps
	ar22 := math.Abs(r22) + eps

	best := math.Inf(-1)

	// --- 3 face axes from A ---
	if g := math.Abs(t0) - hA0 - (hB0*ar00 + hB1*ar01 + hB2*ar02); g > best {
		best = g
	}
	if g := math.Abs(t1) - hA1 - (hB0*ar10 + hB1*ar11 + hB2*ar12); g > best {
		best = g
	}
	if g := math.Abs(t2) - hA2 - (hB0*ar20 + hB1*ar21 + hB2*ar22); g > best {
		best = g
	}

	// --- 3 face axes from B ---
	if g := math.Abs(t0*r00+t1*r10+t2*r20) - hB0 - (hA0*ar00+hA1*ar10+hA2*ar20); g > best {
		best = g
	}
	if g := math.Abs(t0*r01+t1*r11+t2*r21) - hB1 - (hA0*ar01+hA1*ar11+hA2*ar21); g > best {
		best = g
	}
	if g := math.Abs(t0*r02+t1*r12+t2*r22) - hB2 - (hA0*ar02+hA1*ar12+hA2*ar22); g > best {
		best = g
	}

	// --- 9 edge axes (a_i × b_j) with sqrt(1 - R[i][j]^2) normalization ---
	// Skip degenerate (near-parallel) edges where the cross product vanishes.

	// a0 × b0
	if l2 := 1 - r00*r00; l2 > eps {
		raw := math.Abs(t2*r10-t1*r20) - (hA1*ar20 + hA2*ar10) - (hB1*ar02 + hB2*ar01)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a0 × b1
	if l2 := 1 - r01*r01; l2 > eps {
		raw := math.Abs(t2*r11-t1*r21) - (hA1*ar21 + hA2*ar11) - (hB0*ar02 + hB2*ar00)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a0 × b2
	if l2 := 1 - r02*r02; l2 > eps {
		raw := math.Abs(t2*r12-t1*r22) - (hA1*ar22 + hA2*ar12) - (hB0*ar01 + hB1*ar00)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a1 × b0
	if l2 := 1 - r10*r10; l2 > eps {
		raw := math.Abs(t0*r20-t2*r00) - (hA0*ar20 + hA2*ar00) - (hB1*ar12 + hB2*ar11)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a1 × b1
	if l2 := 1 - r11*r11; l2 > eps {
		raw := math.Abs(t0*r21-t2*r01) - (hA0*ar21 + hA2*ar01) - (hB0*ar12 + hB2*ar10)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a1 × b2
	if l2 := 1 - r12*r12; l2 > eps {
		raw := math.Abs(t0*r22-t2*r02) - (hA0*ar22 + hA2*ar02) - (hB0*ar11 + hB1*ar10)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a2 × b0
	if l2 := 1 - r20*r20; l2 > eps {
		raw := math.Abs(t1*r00-t0*r10) - (hA0*ar10 + hA1*ar00) - (hB1*ar22 + hB2*ar21)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a2 × b1
	if l2 := 1 - r21*r21; l2 > eps {
		raw := math.Abs(t1*r01-t0*r11) - (hA0*ar11 + hA1*ar01) - (hB0*ar22 + hB2*ar20)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a2 × b2
	if l2 := 1 - r22*r22; l2 > eps {
		raw := math.Abs(t1*r02-t0*r12) - (hA0*ar12 + hA1*ar02) - (hB0*ar21 + hB1*ar20)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}

	return best
}

// capsuleBoxSATMaxGap computes the maximum separation gap across all 15 SAT axes
// for a capsule vs a box. The capsule is modeled as a degenerate OBB with half-size
// [0, 0, capLen], so all hA0 and hA1 terms are eliminated at compile time.
//
// Input layout (25 float64s, row-major):
//
//	[0..8]   rmA rotation matrix (capsule)
//	[9..17]  rmB rotation matrix (box)
//	[18]     capLen (capsule half-extent along its Z axis = length/2 - radius)
//	[19..21] halfSizeB (box)
//	[22..24] centerDist (boxCenter - capsuleCenter)
//
// The result does NOT include the capsule radius — the caller must subtract it.
func capsuleBoxSATMaxGap(input *[25]float64) float64 {
	const eps = 1e-10

	// Unpack rotation matrices.
	a0, a1, a2 := input[0], input[1], input[2]
	a3, a4, a5 := input[3], input[4], input[5]
	a6, a7, a8 := input[6], input[7], input[8]
	b0, b1, b2 := input[9], input[10], input[11]
	b3, b4, b5 := input[12], input[13], input[14]
	b6, b7, b8 := input[15], input[16], input[17]

	// Capsule half-extent (Z axis only), box half sizes, center distance.
	capLen := input[18]
	hB0, hB1, hB2 := input[19], input[20], input[21]
	cdx, cdy, cdz := input[22], input[23], input[24]

	// T[i] = rmA.Row(i) . centerDist — center distance in capsule's frame.
	t0 := a0*cdx + a1*cdy + a2*cdz
	t1 := a3*cdx + a4*cdy + a5*cdz
	t2 := a6*cdx + a7*cdy + a8*cdz

	// R[i][j] = rmA.Row(i) . rmB.Row(j) — relative rotation.
	r00 := a0*b0 + a1*b1 + a2*b2
	r01 := a0*b3 + a1*b4 + a2*b5
	r02 := a0*b6 + a1*b7 + a2*b8
	r10 := a3*b0 + a4*b1 + a5*b2
	r11 := a3*b3 + a4*b4 + a5*b5
	r12 := a3*b6 + a4*b7 + a5*b8
	r20 := a6*b0 + a7*b1 + a8*b2
	r21 := a6*b3 + a7*b4 + a8*b5
	r22 := a6*b6 + a7*b7 + a8*b8

	// absR[i][j] = |R[i][j]| + eps — epsilon prevents issues with near-parallel edges.
	ar00 := math.Abs(r00) + eps
	ar01 := math.Abs(r01) + eps
	ar02 := math.Abs(r02) + eps
	ar10 := math.Abs(r10) + eps
	ar11 := math.Abs(r11) + eps
	ar12 := math.Abs(r12) + eps
	ar20 := math.Abs(r20) + eps
	ar21 := math.Abs(r21) + eps
	ar22 := math.Abs(r22) + eps

	best := math.Inf(-1)

	// --- 3 face axes from A (capsule frame) ---
	// hA0=0 and hA1=0 drop out; only A[2] retains the capLen term.
	if g := math.Abs(t0) - (hB0*ar00 + hB1*ar01 + hB2*ar02); g > best {
		best = g
	}
	if g := math.Abs(t1) - (hB0*ar10 + hB1*ar11 + hB2*ar12); g > best {
		best = g
	}
	if g := math.Abs(t2) - capLen - (hB0*ar20 + hB1*ar21 + hB2*ar22); g > best {
		best = g
	}

	// --- 3 face axes from B ---
	// hA0*arIJ and hA1*arIJ drop out; only capLen*ar2J remains.
	if g := math.Abs(t0*r00+t1*r10+t2*r20) - hB0 - capLen*ar20; g > best {
		best = g
	}
	if g := math.Abs(t0*r01+t1*r11+t2*r21) - hB1 - capLen*ar21; g > best {
		best = g
	}
	if g := math.Abs(t0*r02+t1*r12+t2*r22) - hB2 - capLen*ar22; g > best {
		best = g
	}

	// --- 9 edge axes (a_i × b_j) ---
	// hA0=0, hA1=0 simplify each edge axis:
	//   a0×bj: only capLen*ar1J remains from capsule side
	//   a1×bj: only capLen*ar0J remains from capsule side
	//   a2×bj: capsule contribution vanishes entirely

	// a0 × b0
	if l2 := 1 - r00*r00; l2 > eps {
		raw := math.Abs(t2*r10-t1*r20) - capLen*ar10 - (hB1*ar02 + hB2*ar01)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a0 × b1
	if l2 := 1 - r01*r01; l2 > eps {
		raw := math.Abs(t2*r11-t1*r21) - capLen*ar11 - (hB0*ar02 + hB2*ar00)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a0 × b2
	if l2 := 1 - r02*r02; l2 > eps {
		raw := math.Abs(t2*r12-t1*r22) - capLen*ar12 - (hB0*ar01 + hB1*ar00)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a1 × b0
	if l2 := 1 - r10*r10; l2 > eps {
		raw := math.Abs(t0*r20-t2*r00) - capLen*ar00 - (hB1*ar12 + hB2*ar11)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a1 × b1
	if l2 := 1 - r11*r11; l2 > eps {
		raw := math.Abs(t0*r21-t2*r01) - capLen*ar01 - (hB0*ar12 + hB2*ar10)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a1 × b2
	if l2 := 1 - r12*r12; l2 > eps {
		raw := math.Abs(t0*r22-t2*r02) - capLen*ar02 - (hB0*ar11 + hB1*ar10)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a2 × b0: capsule terms vanish (hA0*ar10 + hA1*ar00 = 0)
	if l2 := 1 - r20*r20; l2 > eps {
		raw := math.Abs(t1*r00-t0*r10) - (hB1*ar22 + hB2*ar21)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a2 × b1: capsule terms vanish
	if l2 := 1 - r21*r21; l2 > eps {
		raw := math.Abs(t1*r01-t0*r11) - (hB0*ar22 + hB2*ar20)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}
	// a2 × b2: capsule terms vanish
	if l2 := 1 - r22*r22; l2 > eps {
		raw := math.Abs(t1*r02-t0*r12) - (hB0*ar21 + hB1*ar20)
		if g := raw / math.Sqrt(l2); g > best {
			best = g
		}
	}

	return best
}
