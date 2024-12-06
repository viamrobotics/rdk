package cli

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
)

// samePath returns true if abs(path1) and abs(path2) are the same.
func samePath(path1, path2 string) (bool, error) {
	abs1, err := filepath.Abs(path1)
	if err != nil {
		return false, err
	}
	abs2, err := filepath.Abs(path2)
	if err != nil {
		return false, err
	}
	return abs1 == abs2, nil
}

// getMapString is a helper that returns map_[key] if it exists and is a string, otherwise empty string.
func getMapString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return ""
		}
	}
	return ""
}

var fileTypeRegex = regexp.MustCompile(`^[^:]+: ((ELF) [^,]+, ([^,]+),|(Mach-O) [^\ ]+ ([^\ ]+) executable)`)

// ParseFileType parses output from the `file` command. Returns a platform string like "linux/amd64".
// Empty string means failed to parse.
func ParseFileType(raw string) string {
	groups := fileTypeRegex.FindStringSubmatch(raw)
	if len(groups) == 0 {
		return ""
	}
	var rawOs, rawArch string
	if groups[2] != "" {
		rawOs = groups[2]
		rawArch = groups[3]
	} else {
		rawOs = groups[4]
		rawArch = groups[5]
	}
	osLookup := map[string]string{"ELF": "linux", "Mach-O": "darwin"}
	archLookup := map[string]string{"x86-64": "amd64", "ARM aarch64": "arm64", "ARM": "arm32", "x86_64": "amd64", "arm64": "arm64"}
	if archLookup[rawArch] == "arm32" {
		// if we ever parse the different arm versions, give arm32v6 etc. for now, return "" to prevent checking this case.
		return ""
	}
	return fmt.Sprintf("%s/%s", osLookup[rawOs], archLookup[rawArch])
}

func parseBillingAddress(address string) (*apppb.BillingAddress, error) {
	if address == "" {
		return nil, errors.New("address is empty")
	}
	addr := &apppb.BillingAddress{}
	splitAddress := strings.Split(address, ",")
	if len(splitAddress) != 4 || len(splitAddress) != 5 {
		return nil, errors.Errorf("address: %s does not follow the format: line1, line2, city, state, zipcode", address)
	}

	currIndex := 0

	addr.AddressLine_1 = splitAddress[currIndex]
	currIndex++

	if len(splitAddress) == 4 {
		// if its only 4 lines long that means that there is no line2
		currIndex++
	} else {
		addr.AddressLine_2 = &splitAddress[currIndex]
		currIndex++
	}

	addr.City = splitAddress[currIndex]
	currIndex++
	addr.State = splitAddress[currIndex]
	currIndex++
	addr.Zipcode = splitAddress[currIndex]
	return addr, nil
}
