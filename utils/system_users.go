package utils

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Users with these shells exist for system bookkeeping and cannot meaningfully run a module.
var nonLoginShellSuffixes = []string{"nologin", "false", "sync", "shutdown", "halt"}

const nobodyUID = 65534

// parseRunnableUsersFromPasswd returns root plus the regular (uid >= 1000) login users from an /etc/passwd stream.
func parseRunnableUsersFromPasswd(r io.Reader) []string {
	var users []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 7 || fields[0] == "" {
			continue
		}
		uid, err := strconv.Atoi(fields[2])
		if err != nil || uid == nobodyUID || (uid != 0 && uid < 1000) {
			continue
		}
		if isNonLoginShell(fields[6]) {
			continue
		}
		users = append(users, fields[0])
	}
	return users
}

func isNonLoginShell(shell string) bool {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return false
	}
	for _, suffix := range nonLoginShellSuffixes {
		if strings.HasSuffix(shell, suffix) {
			return true
		}
	}
	return false
}
