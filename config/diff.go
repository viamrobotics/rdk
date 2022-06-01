package config

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.viam.com/utils/pexec"
)

// A Diff is the difference between two configs, left and right
// where left is usually old and right is new. So the diff is the
// changes from left to right.
type Diff struct {
	Left, Right    *Config
	Added          *Config
	Modified       *ModifiedConfigDiff
	Removed        *Config
	ResourcesEqual bool
	NetworkEqual   bool
	PrettyDiff     string
}

// ModifiedConfigDiff is the modificative different between two configs.
type ModifiedConfigDiff struct {
	Remotes    []Remote
	Components []Component
	Processes  []pexec.ProcessConfig
	Services   []Service
}

// DiffConfigs returns the difference between the two given configs
// from left to right.
func DiffConfigs(left, right *Config) (*Diff, error) {
	PrettyDiff, err := prettyDiff(left, right)
	if err != nil {
		return nil, err
	}

	diff := Diff{
		Left:       left,
		Right:      right,
		Added:      &Config{},
		Modified:   &ModifiedConfigDiff{},
		Removed:    &Config{},
		PrettyDiff: PrettyDiff,
	}

	// All diffs use the following logic:
	// If left contains something right does not => removed
	// If right contains something left does not => added
	// If left contains something right does and they are not equal => modified
	// If left contains something right does and they are equal => no diff
	// Note: generics would be nice here!
	different := diffRemotes(left.Remotes, right.Remotes, &diff)
	componentsDifferent, err := diffComponents(left.Components, right.Components, &diff)
	if err != nil {
		return nil, err
	}
	different = componentsDifferent || different
	servicesDifferent, err := diffServices(left.Services, right.Services, &diff)
	if err != nil {
		return nil, err
	}
	different = servicesDifferent || different
	different = diffProcesses(left.Processes, right.Processes, &diff) || different
	diff.ResourcesEqual = !different

	networkDifferent := diffNetworkingCfg(left, right)
	diff.NetworkEqual = !networkDifferent

	return &diff, nil
}

func prettyDiff(left, right *Config) (string, error) {
	leftMd, err := json.MarshalIndent(left, "", " ")
	if err != nil {
		return "", err
	}
	rightMd, err := json.MarshalIndent(right, "", " ")
	if err != nil {
		return "", err
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(leftMd), string(rightMd), false)
	filteredDiffs := make([]diffmatchpatch.Diff, 0, len(diffs))
	for _, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		filteredDiffs = append(filteredDiffs, d)
	}
	return dmp.DiffPrettyText(filteredDiffs), nil
}

// String returns a pretty version of the diff.
func (diff *Diff) String() string {
	return diff.PrettyDiff
}

func diffRemotes(left, right []Remote, diff *Diff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]Remote)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			different = diffRemote(l, r, diff) || different
			continue
		}
		diff.Added.Remotes = append(diff.Added.Remotes, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Remotes = append(diff.Removed.Remotes, left[idx])
	}
	return different
}

func diffRemote(left, right Remote, diff *Diff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Remotes = append(diff.Modified.Remotes, right)
	return true
}

func diffComponents(left, right []Component, diff *Diff) (bool, error) {
	leftIndex := make(map[string]int)
	leftM := make(map[string]Component)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			componentDifferent, err := diffComponent(l, r, diff)
			if err != nil {
				return false, err
			}
			different = componentDifferent || different
			continue
		}
		diff.Added.Components = append(diff.Added.Components, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Components = append(diff.Removed.Components, left[idx])
	}
	return different, nil
}

func diffComponent(left, right Component, diff *Diff) (bool, error) {
	if reflect.DeepEqual(left, right) {
		return false, nil
	}
	if left.Type != right.Type || left.SubType != right.SubType {
		return false, fmt.Errorf("cannot modify type of existing component %q", left.Name)
	}
	diff.Modified.Components = append(diff.Modified.Components, right)
	return true, nil
}

func diffProcesses(left, right []pexec.ProcessConfig, diff *Diff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]pexec.ProcessConfig)
	for idx, l := range left {
		leftM[l.ID] = l
		leftIndex[l.ID] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.ID]
		delete(leftM, r.ID)
		if ok {
			different = diffProcess(l, r, diff) || different
			continue
		}
		diff.Added.Processes = append(diff.Added.Processes, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Processes = append(diff.Removed.Processes, left[idx])
	}
	return different
}

func diffProcess(left, right pexec.ProcessConfig, diff *Diff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Processes = append(diff.Modified.Processes, right)
	return true
}

func diffServices(left, right []Service, diff *Diff) (bool, error) {
	leftIndex := make(map[string]int)
	leftM := make(map[string]Service)
	for idx, l := range left {
		leftM[string(l.Type)] = l
		leftIndex[string(l.Type)] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[string(r.Type)]
		delete(leftM, string(r.Type))
		if ok {
			serviceDifferent, err := diffService(l, r, diff)
			if err != nil {
				return false, err
			}
			different = serviceDifferent || different
			continue
		}
		diff.Added.Services = append(diff.Added.Services, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Services = append(diff.Removed.Services, left[idx])
	}
	return different, nil
}

func diffService(left, right Service, diff *Diff) (bool, error) {
	if reflect.DeepEqual(left, right) {
		return false, nil
	}
	if left.Type != right.Type {
		return false, fmt.Errorf("cannot modify type of existing service %q", left.Name)
	}
	diff.Modified.Services = append(diff.Modified.Services, right)
	return true, nil
}

// diffNetworkingCfg returns true if any part of the networking config is different.
func diffNetworkingCfg(left, right *Config) bool {
	if !reflect.DeepEqual(left.Cloud, right.Cloud) {
		return true
	}
	// for network, we have to check each field separately
	if diffNetwork(left.Network, right.Network) {
		return true
	}
	if !reflect.DeepEqual(left.Auth, right.Auth) {
		return true
	}
	return false
}

// diffNetwork returns true if any part of the network config is different.
func diffNetwork(leftCopy, rightCopy NetworkConfig) bool {
	if diffTLS(leftCopy.TLSConfig, rightCopy.TLSConfig) {
		return true
	}

	// TLSConfig holds funcs, which will never deeply equal so ignore them here
	leftCopy.TLSConfig = nil
	rightCopy.TLSConfig = nil

	return !reflect.DeepEqual(leftCopy, rightCopy)
}

// diffTLS returns true if any part of the TLS config is different.
func diffTLS(leftTLS, rightTLS *tls.Config) bool {
	switch {
	case leftTLS == nil && rightTLS == nil:
		return false
	case leftTLS == nil && rightTLS != nil:
	case leftTLS != nil && rightTLS == nil:
		return true
	}

	if leftTLS.MinVersion != rightTLS.MinVersion {
		return true
	}

	leftCert, err := leftTLS.GetCertificate(nil)
	if err != nil {
		return true
	}
	rightCert, err := rightTLS.GetCertificate(nil)
	if err != nil {
		return true
	}
	if !reflect.DeepEqual(leftCert, rightCert) {
		return true
	}
	leftClientCert, err := leftTLS.GetClientCertificate(nil)
	if err != nil {
		return true
	}
	rightClientCert, err := rightTLS.GetClientCertificate(nil)
	if err != nil {
		return true
	}
	if !reflect.DeepEqual(leftClientCert, rightClientCert) {
		return true
	}
	return false
}
