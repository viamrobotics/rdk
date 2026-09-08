package robottestutils

import (
	"errors"
	"syscall"
	"testing"

	pkgerrors "github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"
)

func TestProcessAlreadyGone(t *testing.T) {
	// The shape pexec gives an errno from its kill(2) on the process group. Wrapping it
	// with github.com/pkg/errors is the part worth pinning: processAlreadyGone can only
	// see the errno if that wrapper stays traversable by errors.As.
	asPexecWrapsIt := func(errno syscall.Errno) error {
		return pkgerrors.Wrapf(errno, "error signaling process group %d with signal %s", 1234, "SIGTERM")
	}

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		// What Darwin reports for a group whose only member is an unreaped zombie.
		{"wrapped EPERM", asPexecWrapsIt(syscall.EPERM), true},
		// What both platforms report once the process has been reaped.
		{"wrapped ESRCH", asPexecWrapsIt(syscall.ESRCH), true},
		{"bare ESRCH", syscall.ESRCH, true},
		{"ProcessNotExistsError", &pexec.ProcessNotExistsError{}, true},
		{"unrelated errno", asPexecWrapsIt(syscall.EACCES), false},
		{"unrelated error", errors.New("server failed to stop"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, processAlreadyGone(tc.err), test.ShouldEqual, tc.want)
		})
	}
}
