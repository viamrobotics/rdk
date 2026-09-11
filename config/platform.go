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
	cudaRegex = regexp.MustCompile(`Cuda compilation tools, release (\d+)\.`)
	// l4tCoreVersionRegex captures the L4T major from the nvidia-l4t-core package version,
	// e.g. "36.4.4-20250616085344" -> "36".
	l4tCoreVersionRegex = regexp.MustCompile(`^(\d+)\.`)
	// l4tReleaseRegex captures the L4T (Jetson Linux) major release from the first line of
	// /etc/nv_tegra_release, e.g. "# R36 (release), REVISION: 4.4, ..." -> "36".
	l4tReleaseRegex    = regexp.MustCompile(`# R(\d+) `)
	piModelRegex       = regexp.MustCompile(`Raspberry Pi\s?(Compute Module)?\s?(\d\w*)?\s?(\w+)?\s?(Model (.+))? Rev`)
	darwinVersionRegex = regexp.MustCompile(`(\d+)\.`)
	savedPlatformTags  []string
)

// l4tToJetpack maps an L4T (Jetson Linux) major release to its JetPack major version.
// R32->JetPack 4, R34/R35->JetPack 5, R36->JetPack 6, R38/R39->JetPack 7 (see
// https://developer.nvidia.com/embedded/jetson-linux-archive).
var l4tToJetpack = map[string]string{
	"32": "4",
	"34": "5",
	"35": "5",
	"36": "6",
	"38": "7",
	"39": "7",
}

// helper to read platform tags for GPU-related system libraries.
func readGPUTags(ctx context.Context, logger logging.Logger, tags []string) []string {
	if _, err := exec.LookPath("nvcc"); err == nil {
		out, err := exec.CommandContext(ctx, "nvcc", "--version").Output()
		if err != nil {
			logger.Errorw("error getting Cuda version from nvcc. Cuda-specific modules may not load", "err", err)
		}
		if match := cudaRegex.FindSubmatch(out); match != nil {
			tags = append(tags, "cuda:true", "cuda_version:"+string(match[1]))
		} else {
			logger.Error("error parsing `nvcc --version` output. Cuda-specific modules may not load")
		}
	}
	tags = readJetpackTag(ctx, logger, tags)
	return tags
}

// readJetpackTag adds a `jetpack:<major>` tag derived from the installed L4T (Jetson Linux)
// release, if this is a Jetson. NVIDIA recommends reading the version from the installed
// nvidia-l4t-core package (this is what the jetson-inference install scripts do), so we try that
// first and fall back to /etc/nv_tegra_release, which the L4T BSP writes, when the package query
// is unavailable.
func readJetpackTag(ctx context.Context, logger logging.Logger, tags []string) []string {
	l4tMajor := l4tMajorFromCorePackage(ctx)
	if l4tMajor == "" {
		l4tMajor = l4tMajorFromReleaseFile(logger)
	}
	if l4tMajor == "" {
		// not a Jetson, or couldn't determine the release: no jetpack tag.
		return tags
	}
	jetpack, ok := l4tToJetpack[l4tMajor]
	if !ok {
		logger.Warnw("unrecognized L4T major release; jetpack tag not set", "l4t_major", l4tMajor)
		return tags
	}
	return append(tags, "jetpack:"+jetpack)
}

// l4tMajorFromCorePackage returns the installed L4T major version from the nvidia-l4t-core dpkg
// package (e.g. "36" from "36.4.4-20250616085344"), or "" if it can't be determined.
func l4tMajorFromCorePackage(ctx context.Context) string {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return ""
	}
	out, err := exec.CommandContext(ctx, "dpkg-query", "--showformat=${Version}", "--show", "nvidia-l4t-core").Output()
	if err != nil {
		// a non-zero exit usually means the package isn't installed (i.e. not a Jetson).
		return ""
	}
	if match := l4tCoreVersionRegex.FindSubmatch(out); match != nil {
		return string(match[1])
	}
	return ""
}

// l4tMajorFromReleaseFile returns the L4T major version from /etc/nv_tegra_release (e.g. "36"
// from "# R36 (release), ..."), or "" if the file is missing or unparseable.
func l4tMajorFromReleaseFile(logger logging.Logger) string {
	body, err := os.ReadFile("/etc/nv_tegra_release")
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Errorw("can't read /etc/nv_tegra_release, jetpack modules may not load", "err", err)
		}
		// not a Jetson (file absent), or unreadable.
		return ""
	}
	if match := l4tReleaseRegex.FindSubmatch(body); match != nil {
		return string(match[1])
	}
	logger.Warnw("could not parse L4T release from /etc/nv_tegra_release; jetpack tag not set",
		"contents", string(body))
	return ""
}

type piModel struct {
	version     string
	longVersion string
}

// inner logic for pi version parsing.
func parsePi(logger logging.Logger, raw []byte) *piModel {
	if match := piModelRegex.FindSubmatch(raw); match != nil {
		litePlus := string(match[3])
		cm := string(match[1])
		model := strings.Replace(string(match[5]), " Plus", "p", 1)
		ret := &piModel{
			version: string(match[2]),
		}
		if cm != "" {
			ret.longVersion = "cm"
		}
		if ret.version == "" {
			ret.version = "1"
		}
		ret.longVersion += ret.version
		ret.version = ret.version[:1] // contract 3E to 3 now that it's been copied to longVersion
		switch litePlus {
		case "Lite":
			ret.longVersion += "l"
		case "Plus":
			ret.longVersion += "p"
		case "":
		default:
			logger.Warnw("Lite/Plus token has unexpected value; `pifull` platform tag may be wrong", "value", litePlus)
		}
		ret.longVersion += model
		return ret
	}
	return nil
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
	if model := parsePi(logger, body); model != nil {
		tags = append(tags, "pi:"+model.version)
		tags = append(tags, "pifull:"+model.longVersion)
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

func readDarwinTags(ctx context.Context, logger logging.Logger, tags []string) []string {
	if _, err := exec.LookPath("sw_vers"); err == nil {
		out, err := exec.CommandContext(ctx, "sw_vers", "--productVersion").Output()
		if err != nil {
			logger.Errorw("error getting darwin version from sw_vers. Mac-specific modules may not load", "err", err)
		}
		if match := darwinVersionRegex.FindSubmatch(out); match != nil {
			tags = append(tags, "os_version:"+string(match[1]))
		} else {
			logger.Errorw("error parsing sw_vers version output. Mac-specific modules may not load", "input", string(out))
		}
	}
	return tags
}

// This reads the granular platform constraints (os version, distro, etc).
// This further constrains the basic runtime.GOOS/GOARCH stuff in getAgentInfo
// so module authors can publish builds with ABI or SDK dependencies. The
// list of tags returned by this function is expected to grow.
func readExtendedPlatformTags(logger logging.Logger, cache bool) []string {
	if cache && savedPlatformTags != nil {
		return savedPlatformTags
	}

	// this timeout is for all steps in this function.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	tags := make([]string, 0, 3)

	switch runtime.GOOS {
	case "linux":
		tags = readLinuxTags(logger, tags)
		tags = readGPUTags(ctx, logger, tags)
		tags = readPiTags(logger, tags)
	case "darwin":
		tags = readDarwinTags(ctx, logger, tags)
	}
	if cache {
		savedPlatformTags = tags
		// note: we only log in the cache condition because it would be annoying to log this in a loop.
		platform := runtime.GOOS + "/" + runtime.GOARCH
		logger.Infow("platform detected for machine", "platform", platform, "tags", strings.Join(tags, ","))
	}
	return tags
}
