// Command mcastprobe reports whether the host can join the mDNS multicast group.
// It mirrors the interface selection goutils applies before an mDNS dial, so a
// failure here is the same failure that makes an mDNS name unresolvable.
package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// candidates mirrors the unexported rpc.listMulticastInterfaces in goutils: every
// interface that is up and either loopback or multicast-capable.
func candidates() ([]net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []net.Interface
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 {
			continue
		}
		if ifi.Flags&net.FlagLoopback != 0 || ifi.Flags&net.FlagMulticast != 0 {
			out = append(out, ifi)
		}
	}
	return out, nil
}

type tally struct {
	mu        sync.Mutex
	enumErr   int
	seen      map[string]int
	joined    map[string]int
	refused   map[string]int
	firstErr  map[string]string
	flagsSeen map[string]struct{}
	meta      map[string]string
}

func newTally() *tally {
	return &tally{
		seen:      map[string]int{},
		joined:    map[string]int{},
		refused:   map[string]int{},
		firstErr:  map[string]string{},
		flagsSeen: map[string]struct{}{},
		meta:      map[string]string{},
	}
}

func (t *tally) run(iters int) {
	for i := 0; i < iters; i++ {
		ifaces, err := candidates()
		if err != nil {
			t.mu.Lock()
			t.enumErr++
			t.mu.Unlock()
			continue
		}
		names := make([]string, 0, len(ifaces))
		for j := range ifaces {
			ifi := ifaces[j]
			names = append(names, ifi.Name)
			conn, joinErr := net.ListenMulticastUDP("udp4", &ifi, mdnsGroup)
			t.mu.Lock()
			t.seen[ifi.Name]++
			if _, dup := t.meta[ifi.Name]; !dup {
				t.meta[ifi.Name] = fmt.Sprintf("flags=%s v4=%t", ifi.Flags, hasIPv4(&ifi))
			}
			if joinErr != nil {
				t.refused[ifi.Name]++
				if _, dup := t.firstErr[ifi.Name]; !dup {
					t.firstErr[ifi.Name] = joinErr.Error()
				}
			} else {
				t.joined[ifi.Name]++
			}
			t.mu.Unlock()
			if joinErr == nil {
				conn.Close()
			}
		}
		t.mu.Lock()
		t.flagsSeen[strings.Join(names, ",")] = struct{}{}
		t.mu.Unlock()
	}
}

func hasIPv4(ifi *net.Interface) bool {
	addrs, err := ifi.Addrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			return true
		}
	}
	return false
}

func underQEMU() bool {
	maps, err := os.ReadFile("/proc/self/maps")
	return err == nil && strings.Contains(strings.ToLower(string(maps)), "qemu")
}

func main() {
	label, iters, workers := "probe", 200, 1
	if len(os.Args) > 1 {
		label = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil && n > 0 {
			iters = n
		}
	}
	if len(os.Args) > 3 {
		if n, err := strconv.Atoi(os.Args[3]); err == nil && n > 0 {
			workers = n
		}
	}

	t := newTally()
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.run(iters)
		}()
	}
	wg.Wait()

	fmt.Printf("PROBE label=%s goarch=%s qemu=%t workers=%d iters=%d enum_errors=%d\n",
		label, runtime.GOARCH, underQEMU(), workers, iters, t.enumErr)

	sets := make([]string, 0, len(t.flagsSeen))
	for s := range t.flagsSeen {
		sets = append(sets, s)
	}
	sort.Strings(sets)
	fmt.Printf("PROBE label=%s interface_sets=%q\n", label, sets)

	names := make([]string, 0, len(t.seen))
	for n := range t.seen {
		names = append(names, n)
	}
	sort.Strings(names)
	failed := false
	for _, n := range names {
		fmt.Printf("PROBE label=%s iface=%s %s seen=%d joined=%d refused=%d\n",
			label, n, t.meta[n], t.seen[n], t.joined[n], t.refused[n])
		if t.refused[n] > 0 {
			failed = true
			fmt.Printf("PROBE label=%s iface=%s first_error=%q\n", label, n, t.firstErr[n])
		}
	}
	if failed || t.enumErr > 0 {
		fmt.Printf("PROBE label=%s VERDICT=degraded\n", label)
		return
	}
	fmt.Printf("PROBE label=%s VERDICT=ok\n", label)
}
