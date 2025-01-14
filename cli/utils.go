package cli

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
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

	splitAddress := strings.Split(address, ",")
	if len(splitAddress) != 4 && len(splitAddress) != 5 {
		return nil, errors.Errorf("address: %s does not follow the format: line1, line2 (optional), city, state, zipcode", address)
	}

	if len(splitAddress) == 4 {
		return &apppb.BillingAddress{
			AddressLine_1: strings.Trim(splitAddress[0], " "),
			City:          strings.Trim(splitAddress[1], " "),
			State:         strings.Trim(splitAddress[2], " "),
			Zipcode:       strings.Trim(splitAddress[3], " "),
		}, nil
	}

	line2 := strings.Trim(splitAddress[1], " ")
	return &apppb.BillingAddress{
		AddressLine_1: strings.Trim(splitAddress[0], " "),
		AddressLine_2: &line2,
		City:          strings.Trim(splitAddress[2], " "),
		State:         strings.Trim(splitAddress[3], " "),
		Zipcode:       strings.Trim(splitAddress[4], " "),
	}, nil
}

func parseTimeString(timeStr string) (*timestamppb.Timestamp, error) {
	if timeStr == "" {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse time string: %s", timeStr)
	}

	return timestamppb.New(t), nil
}

func formatStringForOutput(protoString, prefixToTrim string) string {
	return strings.ToLower(strings.TrimPrefix(protoString, prefixToTrim))
}
