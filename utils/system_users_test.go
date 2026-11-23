package utils

import (
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestParseRunnableUsersFromPasswd(t *testing.T) {
	passwd := `# comment
root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
sync:x:4:65534:sync:/bin:/bin/sync
www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
viam:x:1000:1000:Viam:/home/viam:/bin/bash
svc:x:1001:1001::/home/svc:/bin/false
blankshell:x:1002:1002::/home/blankshell:
malformed:x:1003
`
	users := parseRunnableUsersFromPasswd(strings.NewReader(passwd))
	test.That(t, users, test.ShouldResemble, []string{"root", "viam", "blankshell"})
}
