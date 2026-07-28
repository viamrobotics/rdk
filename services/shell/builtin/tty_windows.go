package builtin

import "os/exec"

// setControllingTTY is a no-op on Windows; ConPTY handles the console session.
func setControllingTTY(_ *exec.Cmd) {}
