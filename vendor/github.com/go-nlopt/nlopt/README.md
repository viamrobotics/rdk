A NLopt implementation for Go
======

A package to provide functionality of object-oriented C-API of [NLopt](http://ab-initio.mit.edu/wiki/index.php/Main_Page) 
for the Go programming language (http://golang.org). This provides a wrapper 
using cgo to a c-based implementation.


## Status

[![Build Status](https://travis-ci.org/go-nlopt/nlopt.svg?branch=master)](https://travis-ci.org/go-nlopt/nlopt) [![Coverage Status](https://coveralls.io/repos/github/go-nlopt/nlopt/badge.svg?branch=master)](https://coveralls.io/github/go-nlopt/nlopt?branch=master) [![GoDoc](https://godoc.org/github.com/go-nlopt/nlopt?status.svg)](https://godoc.org/github.com/go-nlopt/nlopt)


## Installation

- On RedHat/CentOS/Fedora

~~~shell script
yum/dnf -y install nlopt-devel
~~~

- On Ubuntu (14.04+)

~~~shell script
apt-get install -y libnlopt-dev
~~~

- or, install NLopt library on any Unix-like system (GNU/Linux is fine) with a 
  C compiler, using the standard procedure:

~~~shell script
curl -O https://codeload.github.com/stevengj/nlopt/tar.gz/v2.7.0 && tar xzvf v2.7.0 && cd nlopt-2.7.0
cmake . && make && sudo make install
~~~

- On Windows download binary packages at [NLopt on Windows](http://ab-initio.mit.edu/wiki/index.php/NLopt_on_Windows)

If you use pre-packaged binaries, you might want to either make symlink or a copy of `libnlopt-0.dll` library
file as `libnlopt.dll`, e.g.:

~~~shell script
mklink libnlopt.dll libnlopt-0.dll
~~~

If the C++ library is in a non-standard directory, or you are using Windows, 
make sure to export `LIBRARY_PATH` environment variable, e.g.:

~~~shell script
export LIBRARY_PATH=/path/to/NLopt
~~~

or, on Windows:

~~~shell script
set LIBRARY_PATH=C:\path\to\NLopt
~~~


Then install `nlopt` package. 

~~~shell script
go get -u github.com/go-nlopt/nlopt
~~~


## Examples

Implementation of nonlinearly constrained problem from [NLopt Tutorial](http://ab-initio.mit.edu/wiki/index.php/NLopt_Tutorial)

~~~go
package main

import (
        "fmt"
        "github.com/go-nlopt/nlopt"
        "math"
)

func main() {
        opt, err := nlopt.NewNLopt(nlopt.LD_MMA, 2)
        if err != nil {
                panic(err)
        }
        defer opt.Destroy()

        opt.SetLowerBounds([]float64{math.Inf(-1), 0.})

        var evals int
        myfunc := func(x, gradient []float64) float64 {
                evals++
                if len(gradient) > 0 {
                        gradient[0] = 0.0
                        gradient[1] = 0.5 / math.Sqrt(x[1])
                }
                return math.Sqrt(x[1])
        }
        
        myconstraint := func(x, gradient []float64, a, b float64) float64 {
                if len(gradient) > 0 {
                        gradient[0] = 3*a* math.Pow(a*x[0]+b, 2.)
                        gradient[1] = -1.0
                }
                return math.Pow(a*x[0]+b, 3) - x[1]
        }

        opt.SetMinObjective(myfunc)
        opt.AddInequalityConstraint(func(x, gradient []float64) float64 { return myconstraint(x, gradient, 2., 0.)}, 1e-8)
        opt.AddInequalityConstraint(func(x, gradient []float64) float64 { return myconstraint(x, gradient, -1., 1.)}, 1e-8)
        opt.SetXtolRel(1e-4)

        x := []float64{1.234, 5.678}
        xopt, minf, err := opt.Optimize(x)
        if err != nil {
                panic(err)
        }
        fmt.Printf("found minimum after %d evaluations at f(%g,%g) = %0.10g\n", evals, xopt[0], xopt[1], minf)
}
~~~

Implementation of [Nonlinear Least Squares Without Jacobian](https://uk.mathworks.com/help/optim/ug/nonlinear-least-squares-with-full-jacobian.html)

~~~go
package main

import (
        "fmt"
        "github.com/go-nlopt/nlopt"
        "math"
)

func main() {
        opt, err := nlopt.NewNLopt(nlopt.LN_BOBYQA, 2)
        if err != nil {
                panic(err)
        }
        defer opt.Destroy()

        k := []float64{1., 2., 3., 4., 5., 6., 7., 8., 9., 10.}
        
        var evals int
        myfun := func(x, gradient []float64) float64 {
                evals++
                f := make([]float64, len(k))
                for i := 0; i < len(k); i++ {
                        f[i] = 2 + 2*k[i] - math.Exp(k[i]*x[0]) - math.Exp(k[i]*x[1])
                }
                var chi2 float64
                for i := 0; i < len(f); i++ {
                        chi2 += (f[i] * f[i])
                }
                return chi2
        }
                  
        opt.SetMinObjective(myfun)
        opt.SetXtolRel(1e-8)
        opt.SetFtolRel(1e-8)

        x := []float64{0.3, 0.4}
        xopt, resnorm, err := opt.Optimize(x)
        if err != nil {
                panic(err)
        }
        fmt.Printf("BOBYQA: found minimum after %d evaluations at f(%g,%g) = %0.10g\n", evals, xopt[0], xopt[1], resnorm)
}
~~~

## License

MIT - see LICENSE for more details.