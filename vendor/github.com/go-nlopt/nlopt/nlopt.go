package nlopt

/*
#cgo CFLAGS: -Os
#cgo windows LDFLAGS: -lnlopt -lm
#cgo !windows LDFLAGS: -lm
#cgo !windows pkg-config: nlopt
#include "nlopt.h"
#include <stdlib.h>

extern double nloptFunc(unsigned n, const double *x, double *gradient, void *func_data);
extern double nloptMfunc(unsigned m, double *resultStatus, unsigned n, const double *x, double *gradient, void *func_data);

double nlopt_func_go(unsigned n, const double *x, double *gradient, void *func_data) {
    return nloptFunc(n, x, gradient, func_data);
}

void nlopt_mfunc_go(unsigned m, double *resultStatus, unsigned n, const double *x, double *gradient, void *func_data) {
    nloptMfunc(m, resultStatus, n, x, gradient, func_data);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"
)

const (
	// Algorithms

	// GN_DIRECT is DIRECT (global, no-derivative)
	GN_DIRECT = iota
	// GN_DIRECT_L is DIRECT-L (global, no-derivative)
	GN_DIRECT_L
	// GN_DIRECT_L_RAND is Randomized DIRECT-L (global, no-derivative)
	GN_DIRECT_L_RAND
	// GN_DIRECT_NOSCAL is Unscaled DIRECT (global, no-derivative)
	GN_DIRECT_NOSCAL
	// GN_DIRECT_L_NOSCAL is Unscaled DIRECT-L (global, no-derivative)
	GN_DIRECT_L_NOSCAL
	// GN_DIRECT_L_RAND_NOSCAL is Unscaled Randomized DIRECT-L (global, no-derivative)
	GN_DIRECT_L_RAND_NOSCAL
	// GN_ORIG_DIRECT is Original DIRECT version (global, no-derivative)
	GN_ORIG_DIRECT
	// GN_ORIG_DIRECT_L is Original DIRECT-L version (global, no-derivative)
	GN_ORIG_DIRECT_L
	// GD_STOGO is StoGO (NOT COMPILED)
	GD_STOGO
	// GD_STOGO_RAND is StoGO randomized (NOT COMPILED)
	GD_STOGO_RAND
	// LD_LBFGS_NOCEDAL is original L-BFGS code by Nocedal et al. (NOT COMPILED)
	LD_LBFGS_NOCEDAL
	// LD_LBFGS is Limited-memory BFGS (L-BFGS) (local, derivative-based)
	LD_LBFGS
	// LN_PRAXIS is Principal-axis, praxis (local, no-derivative)
	LN_PRAXIS
	// LD_VAR1 is Limited-memory variable-metric, rank 1 (local, derivative-based)
	LD_VAR1
	// LD_VAR2 is Limited-memory variable-metric, rank 2 (local, derivative-based)
	LD_VAR2
	// LD_TNEWTON is Truncated Newton (local, derivative-based)
	LD_TNEWTON
	// LD_TNEWTON_RESTART is Truncated Newton with restarting (local, derivative-based)
	LD_TNEWTON_RESTART
	// LD_TNEWTON_PRECOND is Preconditioned truncated Newton (local, derivative-based)
	LD_TNEWTON_PRECOND
	// LD_TNEWTON_PRECOND_RESTART is Preconditioned truncated Newton with restarting (local, derivative-based)
	LD_TNEWTON_PRECOND_RESTART
	// GN_CRS2_LM is Controlled random search (CRS2) with local mutation (global, no-derivative)
	GN_CRS2_LM
	// GN_MLSL is Multi-level single-linkage (MLSL), random (global, no-derivative)
	GN_MLSL
	// GD_MLSL is Multi-level single-linkage (MLSL), random (global, derivative)
	GD_MLSL
	// GN_MLSL_LDS is Multi-level single-linkage (MLSL), quasi-random (global, no-derivative)
	GN_MLSL_LDS
	// GD_MLSL_LDS is Multi-level single-linkage (MLSL), quasi-random (global, derivative)
	GD_MLSL_LDS
	// LD_MMA is Method of Moving Asymptotes (MMA) (local, derivative)
	LD_MMA
	// LN_COBYLA is COBYLA (Constrained Optimization BY Linear Approximations) (local, no-derivative)
	LN_COBYLA
	// LN_NEWUOA is NEWUOA unconstrained optimization via quadratic models (local, no-derivative)
	LN_NEWUOA
	// LN_NEWUOA_BOUND is Bound-constrained optimization via NEWUOA-based quadratic models (local, no-derivative)
	LN_NEWUOA_BOUND
	// LN_NELDERMEAD is Nelder-Mead simplex algorithm (local, no-derivative)
	LN_NELDERMEAD
	// LN_SBPLX is Sbplx variant of Nelder-Mead (re-implementation of Rowan's Subplex) (local, no-derivative)
	LN_SBPLX
	// LN_AUGLAG is Augmented Lagrangian method (local, no-derivative)
	LN_AUGLAG
	// LD_AUGLAG is Augmented Lagrangian method (local, derivative)
	LD_AUGLAG
	// LN_AUGLAG_EQ is Augmented Lagrangian method for equality constraints (local, no-derivative)
	LN_AUGLAG_EQ
	// LD_AUGLAG_EQ is Augmented Lagrangian method for equality constraints (local, derivative)
	LD_AUGLAG_EQ
	// LN_BOBYQA is BOBYQA bound-constrained optimization via quadratic models (local, no-derivative)
	LN_BOBYQA
	// GN_ISRES is ISRES evolutionary constrained optimization (global, no-derivative)
	GN_ISRES
	// AUGLAG is Augmented Lagrangian method (needs sub-algorithm)
	AUGLAG
	// AUGLAG_EQ is Augmented Lagrangian method for equality constraints (needs sub-algorithm)
	AUGLAG_EQ
	// G_MLSL is Multi-level single-linkage (MLSL), random (global, needs sub-algorithm)
	G_MLSL
	// G_MLSL_LDS is Multi-level single-linkage (MLSL), quasi-random (global, needs sub-algorithm)
	G_MLSL_LDS
	// LD_SLSQP is Sequential Quadratic Programming (SQP) (local, derivative)
	LD_SLSQP
	// LD_CCSAQ is CCSA (Conservative Convex Separable Approximations) with simple quadratic approximations (local, derivative)
	LD_CCSAQ
	// GN_ESCH is ESCH evolutionary strategy
	GN_ESCH
	// NUM_ALGORITHMS is number of algorithms
	NUM_ALGORITHMS
)

var (
	algorithms = map[int]C.nlopt_algorithm{
		GN_DIRECT:                  C.NLOPT_GN_DIRECT,
		GN_DIRECT_L:                C.NLOPT_GN_DIRECT_L,
		GN_DIRECT_L_RAND:           C.NLOPT_GN_DIRECT_L_RAND,
		GN_DIRECT_NOSCAL:           C.NLOPT_GN_DIRECT_NOSCAL,
		GN_DIRECT_L_NOSCAL:         C.NLOPT_GN_DIRECT_L_NOSCAL,
		GN_DIRECT_L_RAND_NOSCAL:    C.NLOPT_GN_DIRECT_L_RAND_NOSCAL,
		GN_ORIG_DIRECT:             C.NLOPT_GN_ORIG_DIRECT,
		GN_ORIG_DIRECT_L:           C.NLOPT_GN_ORIG_DIRECT_L,
		GD_STOGO:                   C.NLOPT_GD_STOGO,
		GD_STOGO_RAND:              C.NLOPT_GD_STOGO_RAND,
		LD_LBFGS_NOCEDAL:           C.NLOPT_LD_LBFGS_NOCEDAL,
		LD_LBFGS:                   C.NLOPT_LD_LBFGS,
		LN_PRAXIS:                  C.NLOPT_LN_PRAXIS,
		LD_VAR1:                    C.NLOPT_LD_VAR1,
		LD_VAR2:                    C.NLOPT_LD_VAR2,
		LD_TNEWTON:                 C.NLOPT_LD_TNEWTON,
		LD_TNEWTON_RESTART:         C.NLOPT_LD_TNEWTON_RESTART,
		LD_TNEWTON_PRECOND:         C.NLOPT_LD_TNEWTON_PRECOND,
		LD_TNEWTON_PRECOND_RESTART: C.NLOPT_LD_TNEWTON_PRECOND_RESTART,
		GN_CRS2_LM:                 C.NLOPT_GN_CRS2_LM,
		GN_MLSL:                    C.NLOPT_GN_MLSL,
		GD_MLSL:                    C.NLOPT_GD_MLSL,
		GN_MLSL_LDS:                C.NLOPT_GN_MLSL_LDS,
		GD_MLSL_LDS:                C.NLOPT_GD_MLSL_LDS,
		LD_MMA:                     C.NLOPT_LD_MMA,
		LN_COBYLA:                  C.NLOPT_LN_COBYLA,
		LN_NEWUOA:                  C.NLOPT_LN_NEWUOA,
		LN_NEWUOA_BOUND:            C.NLOPT_LN_NEWUOA_BOUND,
		LN_NELDERMEAD:              C.NLOPT_LN_NELDERMEAD,
		LN_SBPLX:                   C.NLOPT_LN_SBPLX,
		LN_AUGLAG:                  C.NLOPT_LN_AUGLAG,
		LD_AUGLAG:                  C.NLOPT_LD_AUGLAG,
		LN_AUGLAG_EQ:               C.NLOPT_LN_AUGLAG_EQ,
		LD_AUGLAG_EQ:               C.NLOPT_LD_AUGLAG_EQ,
		LN_BOBYQA:                  C.NLOPT_LN_BOBYQA,
		GN_ISRES:                   C.NLOPT_GN_ISRES,
		AUGLAG:                     C.NLOPT_AUGLAG,
		AUGLAG_EQ:                  C.NLOPT_AUGLAG_EQ,
		G_MLSL:                     C.NLOPT_G_MLSL,
		G_MLSL_LDS:                 C.NLOPT_G_MLSL_LDS,
		LD_SLSQP:                   C.NLOPT_LD_SLSQP,
		LD_CCSAQ:                   C.NLOPT_LD_CCSAQ,
		GN_ESCH:                    C.NLOPT_GN_ESCH,
	}
)

// AlgorithmName returns a descriptive string corresponding to a particular
// algorithm `algorithm`
func AlgorithmName(algorithm int) string {
	var cName *C.char = C.nlopt_algorithm_name((C.nlopt_algorithm)(algorithm))
	return C.GoString(cName)
}

// Srand allows to use a "deterministic" sequence of pseudorandom numbers, i.e.
// the same sequence from run to run. For stochastic optimization algorithms, a
// pseudorandom numbers generated by the Mersenne Twister algorithm are used.
// By default, the seed for the random numbers is generated from the system time,
// so that you will get a different sequence of pseudorandom numbers each time
// you run your program.
//
// Some of the algorithms also support using low-discrepancy sequences (LDS),
// sometimes known as quasi-random numbers. NLopt uses the Sobol LDS, which is
// implemented for up to 1111 dimensions.
func Srand(seed uint64) {
	C.nlopt_srand((C.ulong)(seed))
}

// SrandTime resets the seed based on the system time. Normally, you don't need
// to call this as it is called automatically. However, it might be useful if
// you want to "re-randomize" the pseudorandom numbers after calling
// nlopt.Srand to set a deterministic seed.
func SrandTime() {
	C.nlopt_srand_time()
}

// Version determines the version number of NLopt at runtime
func Version() string {
	var major C.int
	var minor C.int
	var bugfix C.int
	C.nlopt_version(&major, &minor, &bugfix)
	return fmt.Sprintf("%d.%d.%d", major, minor, bugfix)
}

// Func is an objective function to minimize or maximize. The return should be
// the value of the function at the point x, where x points to a slice of
// length n of the optimization parameters. If the argument gradient is not
// <nil> or empty then it points to a slice of length n, which should (upon
// return) be set in-place to the gradient of the function with respect to the
// optimization parameters at x
type Func func(x, gradient []float64) float64

// Mfunc is a vector-valued objective function for applications where it is
// more convenient to define a single function that returns the values (and
// gradients) of all constraints at once. Upon return the output value of the
// constraints should be stored in result, a slice of length m (the same as
// the dimension passed to nlopt..Add*MConstraint). In addition, if gradient is
// non-<nil>, then gradient points to a slice of length m*n which should, upon
// return, be set to the gradients of the constraint functions with respect to
// x.
type Mfunc func(result, x, gradient []float64)

// OBJECT-ORIENTED API

// NLopt keeps a C.nlopt_opt "object" (an opaque pointer), then set various
// optimization parameters, and then execute the algorithm.
type NLopt struct {
	cOpt C.nlopt_opt
	dim  uint

	mutex      sync.Mutex
	funcs      []*uint8
	lastStatus string
}

// NewNLopt returns a newly allocated nlopt_opt object given an algorithm
// and the dimensionality of the problem `n` (the number of optimization
// parameters)
func NewNLopt(algorithm int, n uint) (*NLopt, error) {
	if algorithm >= NUM_ALGORITHMS {
		return nil, errors.New("nlopt: invalid algorithm")
	}
	var c_alg C.nlopt_algorithm
	if v, ok := algorithms[algorithm]; ok {
		c_alg = v
	}
	c_opt := C.nlopt_create(c_alg, (C.uint)(n))

	return &NLopt{
		cOpt: c_opt,
		dim:  n,
	}, nil
}

func (n *NLopt) addFunc(f Func) *uint8 {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	ptr := makeFuncPtr(f)
	n.funcs = append(n.funcs, ptr)
	return ptr
}

func (n *NLopt) addMfunc(f Mfunc) *uint8 {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	ptr := makeMfuncPtr(f)
	n.funcs = append(n.funcs, ptr)
	return ptr
}

func (n *NLopt) setStatus(s string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.lastStatus = s
}

func (n *NLopt) cFuncResult(f func() C.nlopt_result) error {
	cResult := f()
	status, err := normResult(cResult)
	n.setStatus(status)
	return err
}

// Destroy deallocates nlopt_opt object and frees all reserved
// resources
func (n *NLopt) Destroy() {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	C.nlopt_destroy(n.cOpt)

	for _, ptr := range n.funcs {
		freeFuncPtr(ptr)
	}
	n.funcs = []*uint8{}
}

// Copy makes an independent copy of an object
func (n *NLopt) Copy() *NLopt {
	c := C.nlopt_copy(n.cOpt)
	return &NLopt{
		cOpt: c,
		dim:  n.dim,
	}
}

// Optimize performs optimization once all of the desired optimization
// parameters have been specified in a given object. Input, x is a slice of
// length n (the dimension of the problem from nlopt..NewNLopt) giving an
// initial guess for the optimization parameters. On successful return,
// a slice contains the optimized values of the parameters, and value contains
// the corresponding value of the objective function.
func (n *NLopt) Optimize(x []float64) ([]float64, float64, error) {
	var cOptF C.double
	cX := toCArray(x)
	var cResult C.nlopt_result = C.nlopt_optimize(n.cOpt, (*C.double)(unsafe.Pointer(&cX[0])), &cOptF)
	status, err := normResult(cResult)
	n.setStatus(status)
	if err != nil {
		return nil, math.NaN(), err
	}
	return toGoArray(cX), float64(cOptF), nil
}

// SetMinObjective sets the objective function f to minimize
func (n *NLopt) SetMinObjective(f Func) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			ptr := n.addFunc(f)
			return C.nlopt_set_min_objective(n.cOpt, (C.nlopt_func)(unsafe.Pointer(C.nlopt_func_go)), unsafe.Pointer(ptr))
		})
}

// SetMaxObjective sets the objective function f to maximize
func (n *NLopt) SetMaxObjective(f Func) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			ptr := n.addFunc(f)
			return C.nlopt_set_max_objective(n.cOpt, (C.nlopt_func)(unsafe.Pointer(C.nlopt_func_go)), unsafe.Pointer(ptr))
		})
}

// GetAlgorithm returns an immutable algorithm id parameter for this instance
func (n *NLopt) GetAlgorithm() int {
	var c_alg C.nlopt_algorithm = C.nlopt_get_algorithm(n.cOpt)
	return int(c_alg)
}

// GetAlgorithm returns a descriptive immutable algorithm name for this
// instance
func (n *NLopt) GetAlgorithmName() string {
	return AlgorithmName(n.GetAlgorithm())
}

// GetDimension returns an immutable dimension parameter for this instance
func (n *NLopt) GetDimension() uint {
	var cDim C.unsigned = C.nlopt_get_dimension(n.cOpt)
	return uint(cDim)
}

func (n *NLopt) LastStatus() string {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	return n.lastStatus
}

// generic algorithm parameters:

// SetParam sets an internal algorithm parameter
func (n *NLopt) SetParam(name string, val float64) error {
	var cResult C.nlopt_result = C.nlopt_set_param(n.cOpt, C.CString(name), (C.double)(val))
	status, err := normResult(cResult)
	n.setStatus(status)
	return err
}

// GetParam gets an internal algorithm parameter by name
func (n *NLopt) GetParam(name string, defaultVal float64) float64 {
	var cResult C.double = C.nlopt_get_param(n.cOpt, C.CString(name), (C.double)(defaultVal))
	return float64(cResult)
}

// HasParam checks whether an internal algorithm parameter has been set
func (n *NLopt) HasParam(name string) int {
	var cResult C.int = C.nlopt_has_param(n.cOpt, C.CString(name))
	return int(cResult)
}

// HasParam returns number of internal algorithm parameters
func (n *NLopt) NumParams() uint {
	var cResult C.uint = C.nlopt_num_params(n.cOpt)
	return uint(cResult)
}

// NthParam gets an internal algorithm parameter for provided index
func (n *NLopt) NthParam(idx uint) string {
	var cResult = C.nlopt_nth_param(n.cOpt, (C.uint)(idx))
	return C.GoString(cResult)
}

// constraints:

// SetLowerBounds sets lower bounds that an objective function and any
// nonlinear constraints will never be evaluated outside of these bounds.
// Bounds are set by passing a slice lb of length n (the dimension of the
// problem)
func (n *NLopt) SetLowerBounds(lb []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			cLB := toCArray(lb)
			return C.nlopt_set_lower_bounds(n.cOpt, (*C.double)(&cLB[0]))
		})
}

// SetLowerBounds1 sets lower bounds to a single constant for all optimization
// parameters
func (n *NLopt) SetLowerBounds1(lb float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_lower_bounds1(n.cOpt, (C.double)(lb))
		})
}

// GetLowerBounds returns lower bounds. It is possible not to have lower bounds set.
// The size of return slice is n (the dimension of the problem)
func (n *NLopt) GetLowerBounds() ([]float64, error) {
	lb := make([]C.double, n.dim)
	var cResult C.nlopt_result = C.nlopt_get_lower_bounds(n.cOpt, (*C.double)(unsafe.Pointer(&lb[0])))
	status, err := normResult(cResult)
	n.setStatus(status)
	if err != nil {
		return nil, err
	}
	return toGoArray(lb), nil
}

// SetUpperBounds sets upper bounds that an objective function and any
// nonlinear constraints will never be evaluated outside of these bounds.
// Bounds are set by passing a slice lb of length n (the dimension of the
// problem)
func (n *NLopt) SetUpperBounds(ub []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			cUB := toCArray(ub)
			return C.nlopt_set_upper_bounds(n.cOpt, (*C.double)(&cUB[0]))
		})
}

// SetLowerBounds1 sets upper bounds to a single constant for all optimization
// parameters
func (n *NLopt) SetUpperBounds1(ub float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_upper_bounds1(n.cOpt, (C.double)(ub))
		})
}

// GetUpperBounds returns upper bounds. It is possible not to have upper bounds set.
// The size of return slice is n (the dimension of the problem)
func (n *NLopt) GetUpperBounds() ([]float64, error) {
	ub := make([]C.double, n.dim)
	var cResult C.nlopt_result = C.nlopt_get_upper_bounds(n.cOpt, (*C.double)(unsafe.Pointer(&ub[0])))
	status, err := normResult(cResult)
	n.setStatus(status)
	if err != nil {
		return nil, err
	}
	return toGoArray(ub), nil
}

// RemoveEqualityConstraints removes all inequality constraints
func (n *NLopt) RemoveInequalityConstraints() error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_remove_inequality_constraints(n.cOpt)
		})
}

// AddInequalityConstraint adds an arbitrary nonlinear inequality constraint fc.
// The functionality is supported by MMA, COBYLA and ORIG_DIRECT algorithms. The
// parameter tol is a tolerance used for the purpose of stopping criteria only.
func (n *NLopt) AddInequalityConstraint(fc Func, tol float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			ptr := n.addFunc(fc)
			return C.nlopt_add_inequality_constraint(n.cOpt, (C.nlopt_func)(unsafe.Pointer(C.nlopt_func_go)), unsafe.Pointer(ptr), (C.double)(tol))
		})
}

// AddInequalityMConstraint adds vector-valued inequality constraint fc. Slice
// tol points to a slice of length m of the tolerances in each constraint
// dimension (or <nil> for zero tolerances)
func (n *NLopt) AddInequalityMConstraint(fc Mfunc, tol []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			ptr := n.addMfunc(fc)
			cTol := toCArray(tol)
			m := len(tol)
			return C.nlopt_add_inequality_mconstraint(n.cOpt, (C.uint)(m), (C.nlopt_func)(unsafe.Pointer(C.nlopt_mfunc_go)), unsafe.Pointer(ptr), (*C.double)(&cTol[0]))
		})
}

// RemoveEqualityConstraints removes all equality constraints
func (n *NLopt) RemoveEqualityConstraints() error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_remove_equality_constraints(n.cOpt)
		})
}

// AddEqualityConstraint adds an arbitrary nonlinear equality constraint h.
// The functionality is supported by ISRES and AUGLAG algorithms. The parameter
// tol is a tolerance used for the purpose of stopping criteria only.
func (n *NLopt) AddEqualityConstraint(h Func, tol float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			ptr := n.addFunc(h)
			return C.nlopt_add_equality_constraint(n.cOpt, (C.nlopt_func)(unsafe.Pointer(C.nlopt_func_go)), unsafe.Pointer(ptr), (C.double)(tol))
		})
}

// AddEqualityMConstraint adds vector-valued equality constraint h. Slice tol
// points to a slice of length m of the tolerances in each constraint dimension
// (or <nil> for zero tolerances)
func (n *NLopt) AddEqualityMConstraint(h Mfunc, tol []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			ptr := n.addMfunc(h)
			cTol := toCArray(tol)
			m := len(tol)
			return C.nlopt_add_equality_mconstraint(n.cOpt, (C.uint)(m), (C.nlopt_func)(unsafe.Pointer(C.nlopt_mfunc_go)), unsafe.Pointer(ptr), (*C.double)(&cTol[0]))
		})
}

// stopping criteria:

// SetStopVal sets a criterion to stop when an objective value of at least
// stopval is found: stop minimizing when an objective value ≤ stopval is found,
// or stop maximizing a value ≥ stopval is found.
func (n *NLopt) SetStopVal(stopval float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_stopval(n.cOpt, (C.double)(stopval))
		})
}

// GetStopVal retrieves the current value for stopval criterion
func (n *NLopt) GetStopVal() float64 {
	var cStopval C.double = C.nlopt_get_stopval(n.cOpt)
	return float64(cStopval)
}

// SetFtolRel sets relative tolerance on function value: stop when an
// optimization step (or an estimate of the optimum) changes the objective
// function value by less than tol multiplied by the absolute value of the
// function value. Criterion is disabled if tol is non-positive.
func (n *NLopt) SetFtolRel(tol float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_ftol_rel(n.cOpt, (C.double)(tol))
		})
}

// GetFtolRel retrieves the current value for relative function value tolerance
// criterion
func (n *NLopt) GetFtolRel() float64 {
	var cTol C.double = C.nlopt_get_ftol_rel(n.cOpt)
	return float64(cTol)
}

// SetFtolAbs sets absolute tolerance on function value: stop when an
// optimization step (or an estimate of the optimum) changes the function value
// by less than tol. Criterion is disabled if tol is non-positive.
func (n *NLopt) SetFtolAbs(tol float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_ftol_abs(n.cOpt, (C.double)(tol))
		})
}

// GetFtolAbs retrieves the current value for absolute function value tolerance
// criterion
func (n *NLopt) GetFtolAbs() float64 {
	var cTol C.double = C.nlopt_get_ftol_abs(n.cOpt)
	return float64(cTol)
}

// SetXtolRel sets relative tolerance on optimization parameters: stop when an
// optimization step (or an estimate of the optimum) changes every parameter by
// less than tol multiplied by the absolute value of the parameter. Criterion
// is disabled if tol is non-positive.
func (n *NLopt) SetXtolRel(tol float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_xtol_rel(n.cOpt, (C.double)(tol))
		})
}

// GetXtolRel retrieves the current value for relative tolerance on optimization
// parameters criterion
func (n *NLopt) GetXtolRel() float64 {
	var cTol C.double = C.nlopt_get_xtol_rel(n.cOpt)
	return float64(cTol)
}

// SetXtolAbs1 sets the absolute tolerances in all n optimization parameters to
// the same value tol. Criterion is disabled if tol is non-positive.
func (n *NLopt) SetXtolAbs1(tol float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_xtol_abs1(n.cOpt, (C.double)(tol))
		})
}

// SetXtolAbs sets absolute tolerances on optimization parameters. tol is a
// slice of length n (the dimension from NewNLopt) giving the tolerances: stop
// when an optimization step (or an estimate of the optimum) changes every
// parameter x[i] by less than tol[i]
func (n *NLopt) SetXtolAbs(tol []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			cTol := toCArray(tol)
			return C.nlopt_set_xtol_abs(n.cOpt, (*C.double)(&cTol[0]))
		})
}

// GetXtolAbs retrieves the current value for absolute tolerances on optimization
// parameters criterion
func (n *NLopt) GetXtolAbs() ([]float64, error) {
	cTol := make([]C.double, n.dim)
	var cResult C.nlopt_result = C.nlopt_get_xtol_abs(n.cOpt, (*C.double)(unsafe.Pointer(&cTol[0])))
	status, err := normResult(cResult)
	n.setStatus(status)
	if err != nil {
		return nil, err
	}
	return toGoArray(cTol), nil
}

// SetMaxEval sets a criterion to stop when the number of function evaluations
// exceeds maxeval. Criterion is disabled if maxeval is non-positive.
func (n *NLopt) SetMaxEval(maxeval int) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_maxeval(n.cOpt, (C.int)(maxeval))
		})
}

// GetMaxEval retrieves the current value for maxeval criterion
func (n *NLopt) GetMaxEval() int {
	var cMaxEval C.int = C.nlopt_get_maxeval(n.cOpt)
	return int(cMaxEval)
}

// SetMaxTime sets a criterion to stop when the optimization time (in seconds)
// exceeds maxtime. Criterion is disabled if maxtime is non-positive.
func (n *NLopt) SetMaxTime(maxtime float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_maxtime(n.cOpt, (C.double)(maxtime))
		})
}

// GetMaxTime retrieves the current value for maxtime criterion
func (n *NLopt) GetMaxTime() float64 {
	var cMaxTime C.double = C.nlopt_get_maxtime(n.cOpt)
	return float64(cMaxTime)
}

// ForceStop allows caller to force the optimization to halt, for some reason
// unknown to NLopt. This causes nlopt..Optimize to halt, returning the
// FORCED_STOP error. It has no effect if not called during nlopt..Optimize.
func (n *NLopt) ForceStop() error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_force_stop(n.cOpt)
		})
}

// SetForceStop sets a forced-stop integer value val, which can be later
// retrieved. Passing val=0 to nlopt..SetForceStop tells NLopt not to
// force a halt.
func (n *NLopt) SetForceStop(val int) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_force_stop(n.cOpt, (C.int)(val))
		})
}

// GetForceStop retrieves last forced-stop value that was set since the last
// nlopt..Optimize. The force-stop value is reset to zero at the beginning of
// nlopt..Optimize.
func (n *NLopt) GetForceStop() int {
	var cVal C.int = C.nlopt_get_force_stop(n.cOpt)
	return int(cVal)
}

// more algorithm-specific parameters

// SetLocalOptimizer sets a different optimization algorithm as a subroutine
// for algorithms like MLSL and AUGLAG. Here localOpt  is another nlopt.NLopt
// object whose parameters are used to determine the local search algorithm,
// its stopping criteria, and other algorithm parameters. (However, the
// objective function, bounds, and nonlinear-constraint parameters of localOpt
// are ignored.) The dimension n of localOpt must match that of opt.
func (n *NLopt) SetLocalOptimizer(localOpt *NLopt) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_local_optimizer(n.cOpt, localOpt.cOpt)
		})
}

// SetPopulation sets an initial "population" of random points x for several of
// the stochastic search algorithms (e.g., CRS, MLSL, and ISRES). By default,
// this initial population size is chosen heuristically in some
// algorithm-specific way. A pop of zero implies that the heuristic default
// will be used.
func (n *NLopt) SetPopulation(pop uint) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_population(n.cOpt, (C.uint)(pop))
		})
}

// GetPopulation retrieves initial "population" of random points x
func (n *NLopt) GetPopulation() uint {
	var cPop C.uint = C.nlopt_get_population(n.cOpt)
	return uint(cPop)
}

// SetVectorStorage for some of the NLopt algorithms that are limited-memory
// "quasi-Newton" algorithms, which "remember" the gradients from a finite
// number M of the previous optimization steps in order to construct an
// approximate 2nd derivative matrix. The bigger M is, the more storage the
// algorithms require, but on the other hand they may converge faster for
// larger M. By default, NLopt chooses a heuristic value of M.
//
// Passing M=0 (the default) tells NLopt to use a heuristic value. By default,
// NLopt currently sets M to 10 or at most 10 MiB worth of vectors, whichever
// is larger.
func (n *NLopt) SetVectorStorage(M uint) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_vector_storage(n.cOpt, (C.uint)(M))
		})
}

// GetVectorStorage retrieves size of vector storage
func (n *NLopt) GetVectorStorage() uint {
	var cDim C.unsigned = C.nlopt_get_vector_storage(n.cOpt)
	return uint(cDim)
}

func (n *NLopt) SetDefaultInitialStep(x []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			cX := toCArray(x)
			return C.nlopt_set_default_initial_step(n.cOpt, (*C.double)(&cX[0]))
		})
}

// SetInitialStep sets initial step size to perturb x by when optimizer begins
// the optimization for derivative-free local-optimization algorithms. This
// step size should be big enough that the value of the objective changes
// significantly, but not too big if you want to find the local optimum nearest
// to x. By default, NLopt chooses this initial step size heuristically from
// the bounds, tolerances, and other information, but this may not always be
// the best choice. Parameter dx is a slice of length n (the dimension of the
// problem from nlopt..NewNLopt) containing the (nonzero) initial step size for
// each component of the optimization parameters x. If you pass <nil> for dx,
// then NLopt will use its heuristics to determine the initial step size.
func (n *NLopt) SetInitialStep(dx []float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			cDx := toCArray(dx)
			return C.nlopt_set_initial_step(n.cOpt, (*C.double)(&cDx[0]))
		})
}

// SetInitialStep1 sets initial step size to perturb x by when optimizer begins
// the optimization for derivative-free local-optimization algorithms to the
// same value in every direction.
func (n *NLopt) SetInitialStep1(dx float64) error {
	return n.cFuncResult(
		func() C.nlopt_result {
			return C.nlopt_set_initial_step1(n.cOpt, (C.double)(dx))
		})
}

// GetInitialStep retrieves the initial step size. The first alice is the same
// as the initial guess that you plan to pass to nlopt.NLopt.Optimize – if you
// have not set the initial step and NLopt is using its heuristics, its
// heuristic step size may depend on the initial x, which is why you must pass
// it here. Both slices are of length n (the dimension of the problem from
// nlopt.NewNLopt), where the latter on successful return contains the initial
// step sizes.
func (n *NLopt) GetInitialStep() ([]float64, []float64, error) {
	x := make([]C.double, n.dim)
	dx := make([]C.double, n.dim)
	var cResult C.nlopt_result = C.nlopt_get_initial_step(n.cOpt, (*C.double)(&x[0]), (*C.double)(&dx[0]))
	status, err := normResult(cResult)
	n.setStatus(status)
	if err != nil {
		return nil, nil, err
	}
	return toGoArray(x), toGoArray(dx), nil
}

func normResult(code C.nlopt_result) (s string, err error) {
	s = resultStatus(code)
	if code < 0 {
		err = fmt.Errorf("nlopt: %s", s)
	}
	return
}

func resultStatus(code C.nlopt_result) string {
	var s string
	switch code {
	case C.NLOPT_FAILURE:
		// generic failure code
		s = "FAILURE"
	case C.NLOPT_INVALID_ARGS:
		s = "INVALID_ARGS"
	case C.NLOPT_OUT_OF_MEMORY:
		s = "OUT_OF_MEMORY"
	case C.NLOPT_ROUNDOFF_LIMITED:
		s = "ROUNDOFF_LIMITED"
	case C.NLOPT_FORCED_STOP:
		s = "FORCED_STOP"
	case C.NLOPT_SUCCESS:
		// generic success code
		s = "SUCCESS"
	case C.NLOPT_STOPVAL_REACHED:
		s = "STOPVAL_REACHED"
	case C.NLOPT_FTOL_REACHED:
		s = "FTOL_REACHED"
	case C.NLOPT_XTOL_REACHED:
		s = "XTOL_REACHED"
	case C.NLOPT_MAXEVAL_REACHED:
		s = "MAXEVAL_REACHED"
	case C.NLOPT_MAXTIME_REACHED:
		s = "MAXTIME_REACHED"
	}
	return s
}

func toCArray(x []float64) []C.double {
	v := make([]C.double, len(x))
	for i := 0; i < len(x); i++ {
		v[i] = (C.double)(x[i])
	}
	return v
}

func toGoArray(x []C.double) []float64 {
	v := make([]float64, len(x))
	for i := 0; i < len(x); i++ {
		v[i] = float64(x[i])
	}
	return v
}
