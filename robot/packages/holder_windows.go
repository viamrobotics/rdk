//go:build windows

package packages

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Restart Manager constants.
// https://learn.microsoft.com/en-us/windows/win32/api/restartmanager/
const (
	rmSessionKeyLen = 32  // CCH_RM_SESSION_KEY
	cchRmMaxAppName = 255 // CCH_RM_MAX_APP_NAME
	cchRmMaxSvcName = 63  // CCH_RM_MAX_SVC_NAME
	rmErrorMoreData = 234 // ERROR_MORE_DATA
)

// rmUniqueProcess mirrors RM_UNIQUE_PROCESS.
type rmUniqueProcess struct {
	ProcessID        uint32
	ProcessStartTime windows.Filetime
}

// rmProcessInfo mirrors RM_PROCESS_INFO. Field order and the fixed-size arrays must match the C
// layout exactly so the Restart Manager writes into the right offsets.
type rmProcessInfo struct {
	Process          rmUniqueProcess
	AppName          [cchRmMaxAppName + 1]uint16
	ServiceShortName [cchRmMaxSvcName + 1]uint16
	ApplicationType  uint32
	AppStatus        uint32
	TSSessionID      uint32
	Restartable      int32
}

var (
	modRstrtmgr             = windows.NewLazySystemDLL("rstrtmgr.dll")
	procRmStartSession      = modRstrtmgr.NewProc("RmStartSession")
	procRmRegisterResources = modRstrtmgr.NewProc("RmRegisterResources")
	procRmGetList           = modRstrtmgr.NewProc("RmGetList")
	procRmEndSession        = modRstrtmgr.NewProc("RmEndSession")
)

// describePathHolder asks the Windows Restart Manager which process(es) currently hold path open
// and returns e.g. "rustdesk.exe (pid 1234)". It's diagnostic only — it never kills anything.
// Unlike Sysinternals handle64, this runs in-process and needs no extra install or elevation trick
// (viam-server as a LocalSystem service already has the rights). Returns "" on any failure so the
// caller falls back to generic wording rather than surfacing a syscall error.
func describePathHolder(path string) string {
	pids, err := processesHoldingPath(path)
	if err != nil || len(pids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		name := processName(pid)
		if name == "" {
			name = "unknown process"
		}
		parts = append(parts, fmt.Sprintf("%s (pid %d)", name, pid))
	}
	return strings.Join(parts, ", ")
}

// processesHoldingPath returns the PIDs of every process the Restart Manager reports as using path.
func processesHoldingPath(path string) ([]uint32, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var session uint32
	sessionKey := make([]uint16, rmSessionKeyLen+1)
	if ret, _, _ := procRmStartSession.Call(
		uintptr(unsafe.Pointer(&session)),
		0,
		uintptr(unsafe.Pointer(&sessionKey[0])),
	); ret != 0 {
		return nil, fmt.Errorf("RmStartSession failed: %d", ret)
	}
	defer procRmEndSession.Call(uintptr(session)) //nolint:errcheck

	files := []*uint16{pathPtr}
	if ret, _, _ := procRmRegisterResources.Call(
		uintptr(session),
		uintptr(len(files)),
		uintptr(unsafe.Pointer(&files[0])),
		0, 0, 0, 0,
	); ret != 0 {
		return nil, fmt.Errorf("RmRegisterResources failed: %d", ret)
	}

	var needed, count, rebootReasons uint32
	// First call with a nil buffer to learn how many entries the Restart Manager needs.
	if ret, _, _ := procRmGetList.Call(
		uintptr(session),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&count)),
		0,
		uintptr(unsafe.Pointer(&rebootReasons)),
	); ret != 0 && ret != rmErrorMoreData {
		return nil, fmt.Errorf("RmGetList failed: %d", ret)
	}
	if needed == 0 {
		return nil, nil
	}

	infos := make([]rmProcessInfo, needed)
	count = needed
	if ret, _, _ := procRmGetList.Call(
		uintptr(session),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&infos[0])),
		uintptr(unsafe.Pointer(&rebootReasons)),
	); ret != 0 {
		return nil, fmt.Errorf("RmGetList failed: %d", ret)
	}

	pids := make([]uint32, 0, count)
	for i := uint32(0); i < count && i < needed; i++ {
		pids = append(pids, infos[i].Process.ProcessID)
	}
	return pids, nil
}

// processName resolves a PID to its executable's base name (e.g. "rustdesk.exe"), or "" on failure.
func processName(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle) //nolint:errcheck

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:size]))
}
