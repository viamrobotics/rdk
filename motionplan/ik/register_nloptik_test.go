// External test package so the blank import doesn't create an import cycle
// (nloptik imports ik). The init() of nloptik runs before any test in this
// directory, registering the gradient-descent solver factory.
package ik_test

import _ "go.viam.com/rdk/motionplan/ik/nloptik"
