package config

import (
	"crypto/tls"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/resource"
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
	Components []resource.Config
	Processes  []pexec.ProcessConfig
	Services   []resource.Config
	Packages   []PackageConfig
	Modules    []Module
}

// DiffConfigs returns the difference between the two given configs
// from left to right.
func DiffConfigs(left, right Config, revealSensitiveConfigDiffs bool) (_ *Diff, err error) {
	var PrettyDiff string
	if revealSensitiveConfigDiffs {
		PrettyDiff, err = prettyDiff(left, right)
		if err != nil {
			return nil, err
		}
	}

	diff := Diff{
		Left:       &left,
		Right:      &right,
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
	componentsDifferent := diffComponents(left.Components, right.Components, &diff)
	different = componentsDifferent || different
	servicesDifferent := diffServices(left.Services, right.Services, &diff)

	different = servicesDifferent || different
	processesDifferent := diffProcesses(left.Processes, right.Processes, &diff) || different

	different = processesDifferent || different
	packagesDifferent := diffPackages(left.Packages, right.Packages, &diff) || different

	different = packagesDifferent || different
	different = diffModules(left.Modules, right.Modules, &diff) || different

	diff.ResourcesEqual = !different

	networkDifferent := diffNetworkingCfg(&left, &right)
	diff.NetworkEqual = !networkDifferent

	return &diff, nil
}

func prettyDiff(left, right Config) (string, error) {
	leftMd, err := json.Marshal(left)
	if err != nil {
		return "", err
	}
	rightMd, err := json.Marshal(right)
	if err != nil {
		return "", err
	}
	var leftClone, rightClone Config
	if err := json.Unmarshal(leftMd, &leftClone); err != nil {
		return "", err
	}
	if err := json.Unmarshal(rightMd, &rightClone); err != nil {
		return "", err
	}
	left = leftClone
	right = rightClone

	mask := "******"
	sanitizeConfig := func(conf *Config) {
		// Note(erd): keep in mind this will destroy the actual pretty diffing of these which
		// is fine because we aren't considering pretty diff changes to these fields at this level
		// of the stack.
		if conf.Cloud != nil {
			if conf.Cloud.Secret != "" {
				conf.Cloud.Secret = mask
			}
			if conf.Cloud.LocationSecret != "" {
				conf.Cloud.LocationSecret = mask
			}
			for i := range conf.Cloud.LocationSecrets {
				if conf.Cloud.LocationSecrets[i].Secret != "" {
					conf.Cloud.LocationSecrets[i].Secret = mask
				}
			}
			// Not really a secret but annoying to diff
			if conf.Cloud.TLSCertificate != "" {
				conf.Cloud.TLSCertificate = mask
			}
			if conf.Cloud.TLSPrivateKey != "" {
				conf.Cloud.TLSPrivateKey = mask
			}
		}
		for _, hdlr := range conf.Auth.Handlers {
			for key := range hdlr.Config {
				hdlr.Config[key] = mask
			}
		}
		for i := range conf.Remotes {
			rem := &conf.Remotes[i]
			if rem.Secret != "" {
				rem.Secret = mask
			}
			if rem.Auth.Credentials != nil {
				rem.Auth.Credentials.Payload = mask
			}
			if rem.Auth.SignalingCreds != nil {
				rem.Auth.SignalingCreds.Payload = mask
			}
		}
	}
	sanitizeConfig(&left)
	sanitizeConfig(&right)

	leftMd, err = json.MarshalIndent(left, "", " ")
	if err != nil {
		return "", err
	}
	rightMd, err = json.MarshalIndent(right, "", " ")
	if err != nil {
		return "", err
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(leftMd), string(rightMd), true)
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

//nolint:dupl
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
	if left.Equals(right) {
		return false
	}
	diff.Modified.Remotes = append(diff.Modified.Remotes, right)
	return true
}

//nolint:dupl
func diffComponents(left, right []resource.Config, diff *Diff) bool {
	leftIndex := make(map[resource.Name]int)
	leftM := make(map[resource.Name]resource.Config)
	for idx, l := range left {
		leftM[l.ResourceName()] = l
		leftIndex[l.ResourceName()] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.ResourceName()]
		delete(leftM, r.ResourceName())
		if ok {
			componentDifferent := diffComponent(l, r, diff)
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
	return different
}

func diffComponent(left, right resource.Config, diff *Diff) bool {
	if left.Equals(right) {
		return false
	}
	diff.Modified.Components = append(diff.Modified.Components, right)
	return true
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
	if left.Equals(right) {
		return false
	}
	diff.Modified.Processes = append(diff.Modified.Processes, right)
	return true
}

//nolint:dupl
func diffPackages(left, right []PackageConfig, diff *Diff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]PackageConfig)
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
			different = diffPackage(l, r, diff) || different
			continue
		}
		diff.Added.Packages = append(diff.Added.Packages, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Packages = append(diff.Removed.Packages, left[idx])
	}
	return different
}

func diffPackage(left, right PackageConfig, diff *Diff) bool {
	if left.Equals(right) {
		return false
	}
	diff.Modified.Packages = append(diff.Modified.Packages, right)
	return true
}

//nolint:dupl
func diffServices(left, right []resource.Config, diff *Diff) bool {
	leftIndex := make(map[resource.Name]int)
	leftM := make(map[resource.Name]resource.Config)
	for idx, l := range left {
		leftM[l.ResourceName()] = l
		leftIndex[l.ResourceName()] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.ResourceName()]
		delete(leftM, r.ResourceName())
		if ok {
			serviceDifferent := diffService(l, r, diff)
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
	return different
}

func diffService(left, right resource.Config, diff *Diff) bool {
	if left.Equals(right) {
		return false
	}
	diff.Modified.Services = append(diff.Modified.Services, right)
	return true
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
		fallthrough
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

//nolint:dupl
func diffModules(leftModules, rightModules []Module, diff *Diff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]Module)
	for idx, l := range leftModules {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range rightModules {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			different = diffModule(l, r, diff) || different
			continue
		}
		diff.Added.Modules = append(diff.Added.Modules, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Modules = append(diff.Removed.Modules, leftModules[idx])
	}
	return different
}

func diffModule(left, right Module, diff *Diff) bool {
	if left.Equals(right) {
		return false
	}
	diff.Modified.Modules = append(diff.Modified.Modules, right)
	return true
}
