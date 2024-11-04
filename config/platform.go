package config

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
)

var (
	cudaRegex            = regexp.MustCompile(`Cuda compilation tools, release (\d+)\.`)
	aptCacheVersionRegex = regexp.MustCompile(`\nVersion: (\d+)\D`)
	piModelRegex         = regexp.MustCompile(`Raspberry Pi (\d+)`)
	savedPlatformTags    []string
)

// helper to read platform tags for GPU-related system libraries.
func readGPUTags(logger logging.Logger, tags []string) []string {
	// this timeout is for all steps in this function.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := exec.LookPath("nvcc"); err == nil {
		out, err := exec.CommandContext(ctx, "nvcc", "--version").Output()
		if err != nil {
			logger.Errorw("error getting Cuda version from nvcc. Cuda-specific modules may not load", "err", err)
		}
		if match := cudaRegex.FindSubmatch(out); match != nil {
			tags = append(tags, "cuda:true", "cuda_version:"+string(match[1]))
		} else {
			logger.Errorw("error parsing `nvcc --version` output. Cuda-specific modules may not load")
		}
	}
	if _, err := exec.LookPath("apt-cache"); err == nil {
		out, err := exec.CommandContext(ctx, "apt-cache", "show", "nvidia-jetpack").Output()
		// note: the error case here will usually mean 'package missing', we don't analyze it.
		if err == nil {
			if match := aptCacheVersionRegex.FindSubmatch(out); match != nil {
				tags = append(tags, "jetpack:"+string(match[1]))
			}
		}
	}
	return tags
}

// helper to add raspberry pi tags to the list.
func readPiTags(logger logging.Logger, tags []string) []string {
	body, err := os.ReadFile("/proc/device-tree/model")
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Errorw("can't open /proc/device-tree/model, modules may not load correctly", "err", err)
		}
		return tags
	}
	if match := piModelRegex.FindSubmatch(body); match != nil {
		tags = append(tags, "pi:"+string(match[1]))
	}
	return tags
}

// helper to parse the /etc/os-release file on linux systems.
func parseOsRelease(body *bufio.Reader) map[string]string {
	ret := make(map[string]string)
	for {
		line, err := body.ReadString('\n')
		if err != nil {
			return ret
		}
		key, value, _ := strings.Cut(line, "=")
		// note: we trim `value` rather than `line` because os_version value is quoted sometimes.
		ret[key] = strings.Trim(value, "\n\"")
	}
}

// append key:value pair to orig if value is non-empty.
func appendPairIfNonempty(orig []string, key, value string) []string {
	if value != "" {
		return append(orig, key+":"+value)
	}
	return orig
}

// helper to tag-ify the contents of /etc/os-release.
func readLinuxTags(logger logging.Logger, tags []string) []string {
	if body, err := os.Open("/etc/os-release"); err != nil {
		if !os.IsNotExist(err) {
			logger.Errorw("can't open /etc/os-release, modules may not load correctly", "err", err)
		}
	} else {
		defer body.Close() //nolint:errcheck
		osRelease := parseOsRelease(bufio.NewReader(body))
		tags = appendPairIfNonempty(tags, "distro", osRelease["ID"])
		tags = appendPairIfNonempty(tags, "os_version", osRelease["VERSION_ID"])
		tags = appendPairIfNonempty(tags, "codename", osRelease["VERSION_CODENAME"])
	}
	return tags
}

// This reads the granular platform constraints (os version, distro, etc).
// This further constrains the basic runtime.GOOS/GOARCH stuff in getAgentInfo
// so module authors can publish builds with ABI or SDK dependencies. The
// list of tags returned by this function is expected to grow.
func readExtendedPlatformTags(logger logging.Logger, cache bool) []string {
	// TODO(APP-6696): CI in multiple environments (alpine + mac), darwin support.
	if cache && savedPlatformTags != nil {
		return savedPlatformTags
	}
	tags := make([]string, 0, 3)
	if runtime.GOOS == "linux" {
		tags = readLinuxTags(logger, tags)
		tags = readGPUTags(logger, tags)
		tags = readPiTags(logger, tags)
	}
	if cache {
		savedPlatformTags = tags
		// note: we only log in the cache condition because it would be annoying to log this in a loop.
		logger.Infow("platform tags", "tags", strings.Join(tags, ","))
	}
	return tags
}
