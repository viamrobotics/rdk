package capture

import (
	"context"
	"fmt"
	"slices"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

// collectorUpdate describes one pending change to the collectors map.
type collectorUpdate struct {
	md  collectorMetadata
	cac *collectorAndConfig
}

// effectiveCollectorConfig is the merged (default + sensor override) state for a single
// collector — i.e. what should be running right now.
type effectiveCollectorConfig struct {
	res resource.Resource
	cfg datamanager.DataCaptureConfig
	key string
}

// DataCaptureConfigKey returns the lookup key for a per-resource capture config map.
func DataCaptureConfigKey(resourceString, method string) string {
	return fmt.Sprintf("%s/%s", resourceString, method)
}

// SetCaptureConfigs applies dynamic per-resource capture configs without triggering a full Reconfigure.
// Only collectors whose effective config (default + override) has changed are restarted.
// Passing nil or an empty map reverts all collectors to their default machine configs.
//
// Overrides may target resource/method pairs that are not in the static config. When that happens,
// the resource is looked up by short name in c.resourcesByShortName and a collector is built from
// the sensor reading. Methods that require additional_params (e.g. board.Analogs, board.Gpios)
// cannot be enabled this way; they must remain in the static machine config.
func (c *Capture) SetCaptureConfigs(ctx context.Context, captureConfigReadings map[string]datamanager.CaptureConfigReading) {
	effectiveCollectors := c.buildEffectiveCollectors(captureConfigReadings)
	c.updateCollectors(effectiveCollectors)
}

// buildEffectiveCollectors computes the merged state of every collector that should be running,
// from static defaults and the sensor's current reading.
func (c *Capture) buildEffectiveCollectors(
	captureConfigReadings map[string]datamanager.CaptureConfigReading,
) map[collectorMetadata]effectiveCollectorConfig {
	effectiveCollectors := map[collectorMetadata]effectiveCollectorConfig{}
	defaultKeys := map[string]struct{}{}

	// 1. iterate through all resources with data capture configured, apply sensor reading overrides.
	for res, defaultCollectorConfigsByResource := range c.defaultCollectorConfigs {
		for _, defaultCfg := range defaultCollectorConfigsByResource {
			key := DataCaptureConfigKey(defaultCfg.Name.ShortName(), defaultCfg.Method)
			defaultKeys[key] = struct{}{}

			// Start from a copy of default config.
			effectiveCfg := defaultCfg

			// Apply override if present, otherwise use default config as-is.
			if override, ok := captureConfigReadings[key]; ok {
				applyOverride(&effectiveCfg, override)
			}
			if effectivelyDisabled(effectiveCfg) {
				continue
			}
			effectiveCollectors[newCollectorMetadata(effectiveCfg)] = effectiveCollectorConfig{res, effectiveCfg, key}
		}
	}

	// 2. iterate through sensor readings to override resources that dont have data capture configured.
	for key, override := range captureConfigReadings {
		// Skip readings whose key matches a static default — those were already merged in step 1.
		if _, seen := defaultKeys[key]; seen {
			continue
		}
		res, ok := c.resourcesByShortName[override.ResourceName]
		if !ok {
			c.logger.Warnw("capture control sensor referenced unknown resource",
				"resource", override.ResourceName, "method", override.MethodName)
			continue
		}
		// Synthesize a fresh DataCaptureConfig from the resolved resource and the sensor
		// reading. The static config didn't provide one for this key, so this is "auto-enable."
		effectiveCfg := datamanager.DataCaptureConfig{
			Name:             res.Name(),
			Method:           override.MethodName,
			CaptureDirectory: c.captureDir,
		}
		applyOverride(&effectiveCfg, override)
		if effectivelyDisabled(effectiveCfg) {
			continue
		}
		effectiveCollectors[newCollectorMetadata(effectiveCfg)] = effectiveCollectorConfig{res, effectiveCfg, key}
	}

	return effectiveCollectors
}

// effectivelyDisabled returns true when this config should not produce a running collector —
// either because Disabled is set, or because the frequency is so close to zero that
// data.GetDurationFromHz rounds to a non-positive interval.
func effectivelyDisabled(cfg datamanager.DataCaptureConfig) bool {
	return cfg.Disabled || data.GetDurationFromHz(cfg.CaptureFrequencyHz) <= 0
}

// updateCollectors makes c.collectors match effectiveCollectors: build/update what's in effectiveCollectors but
// not running correctly, close what's running but not in effectiveCollectors.
func (c *Capture) updateCollectors(effectiveCollectors map[collectorMetadata]effectiveCollectorConfig) {
	var toClose []*collectorAndConfig
	var updates []collectorUpdate

	// Build or update collectors that should be running.
	for metadata, effective := range effectiveCollectors {
		res, effectiveCfg, key := effective.res, effective.cfg, effective.key
		existing := c.collectors[metadata]
		// Skip if the effective config is unchanged.
		if existing != nil && res == existing.Resource && captureConfigUnchanged(existing.Config, effectiveCfg) {
			continue
		}

		// Rebuild collectors to reflect override changes.
		c.logCaptureConfigChange(key, existing, effectiveCfg)
		coll, err := c.buildCollector(res, metadata, effectiveCfg, c.maxCaptureFileSize, c.mongo.collection)
		if err != nil {
			c.logger.Warnw("failed to build collector", "error", err, "key", key)
			continue
		}
		if coll == nil {
			// Defensive: buildEffectiveCollectors's effectivelyDisabled filter should have prevented this.
			continue
		}
		if existing != nil {
			toClose = append(toClose, existing)
		}
		updates = append(updates, collectorUpdate{metadata, coll})
	}

	// Close collectors that should no longer be running (disabled by override or previously
	// enabled by a sensor override that the sensor has now dropped).
	for metadata, existing := range c.collectors {
		if _, ok := effectiveCollectors[metadata]; ok {
			continue
		}
		key := DataCaptureConfigKey(existing.Config.Name.ShortName(), existing.Config.Method)
		c.logger.Infof("capture control sensor disabling capture for %s", key)
		toClose = append(toClose, existing)
		updates = append(updates, collectorUpdate{metadata, nil})
	}

	if len(updates) == 0 {
		return
	}

	// Close old collectors and update the collectors map atomically.
	c.collectorsMu.Lock()
	defer c.collectorsMu.Unlock()
	for _, old := range toClose {
		old.Collector.Close()
	}

	for _, u := range updates {
		if u.cac != nil {
			c.collectors[u.md] = u.cac
		} else {
			delete(c.collectors, u.md)
		}
	}
}

// applyOverride mutates cfg in place with whichever fields the sensor reading provides.
// Fields that are nil on the reading leave cfg untouched.
func applyOverride(cfg *datamanager.DataCaptureConfig, override datamanager.CaptureConfigReading) {
	if override.CaptureFrequencyHz != nil {
		cfg.CaptureFrequencyHz = *override.CaptureFrequencyHz
		cfg.Disabled = *override.CaptureFrequencyHz <= 0
	}
	if override.Tags != nil {
		cfg.Tags = override.Tags
	}
}

// captureConfigUnchanged returns true when CaptureFrequencyHz, Disabled, and Tags (the only fields
// that can be changed via the capture_control_sensor) are identical between an existing and new
// collector config.
func captureConfigUnchanged(existing, effective datamanager.DataCaptureConfig) bool {
	return existing.CaptureFrequencyHz == effective.CaptureFrequencyHz &&
		existing.Disabled == effective.Disabled &&
		slices.Equal(existing.Tags, effective.Tags)
}

// logCaptureConfigChange logs changes between an existing collector's config
// and the newly computed config.
func (c *Capture) logCaptureConfigChange(key string, existing *collectorAndConfig, effectiveCfg datamanager.DataCaptureConfig) {
	if existing == nil {
		c.logger.Infof("capture control sensor enabling capture for %s: capture_frequency_hz=%f tags=%v",
			key, effectiveCfg.CaptureFrequencyHz, effectiveCfg.Tags)
		return
	}
	prev := existing.Config
	if prev.CaptureFrequencyHz != effectiveCfg.CaptureFrequencyHz {
		c.logger.Infof("capture control sensor changing capture_frequency_hz for %s: %f -> %f",
			key, prev.CaptureFrequencyHz, effectiveCfg.CaptureFrequencyHz)
	}
	if !slices.Equal(prev.Tags, effectiveCfg.Tags) {
		c.logger.Infof("capture control sensor changing tags for %s: %v -> %v", key, prev.Tags, effectiveCfg.Tags)
	}
}
