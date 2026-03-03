package capture

import (
	"context"
	"fmt"
	"slices"

	"go.viam.com/rdk/services/datamanager"
)

// DataCaptureConfigKey returns the lookup key for a per-resource capture config map.
func DataCaptureConfigKey(resourceString, method string) string {
	return fmt.Sprintf("%s/%s", resourceString, method)
}

// SetCaptureConfigs applies dynamic per-resource capture configs without triggering a full Reconfigure.
// Only collectors whose effective config (default + override) has changed are restarted.
// Passing nil or an empty map reverts all collectors to their default machine configs.
// configs is keyed by "resourceName/method" (e.g. "camera-1/GetImages").
func (c *Capture) SetCaptureConfigs(ctx context.Context, captureConfigReadings map[string]datamanager.CaptureConfigReading) {
	type collectorUpdate struct {
		md  collectorMetadata
		cac *collectorAndConfig // nil means remove
	}
	var toClose []*collectorAndConfig
	var updates []collectorUpdate

	for res, defaultCollectorConfigs := range c.defaultCollectorConfigs {
		for _, defaultCfg := range defaultCollectorConfigs {
			key := DataCaptureConfigKey(defaultCfg.Name.ShortName(), defaultCfg.Method)

			// Start from a copy of default config.
			effectiveCfg := defaultCfg

			// Apply override if present, otherwise use default config as-is.
			if override, ok := captureConfigReadings[key]; ok {
				if override.CaptureFrequencyHz != nil {
					effectiveCfg.CaptureFrequencyHz = *override.CaptureFrequencyHz
					effectiveCfg.Disabled = *override.CaptureFrequencyHz <= 0
				}
				if override.Tags != nil {
					effectiveCfg.Tags = override.Tags
				}
			}

			md := newCollectorMetadata(effectiveCfg)
			existing := c.collectors[md]

			if effectiveCfg.Disabled {
				if existing != nil {
					c.logger.Infof("capture config disabling capture for %s", key)
					toClose = append(toClose, existing)
					updates = append(updates, collectorUpdate{md, nil})
				}
				continue
			}

			// Skip if the effective config is unchanged.
			if existing != nil && res == existing.Resource && captureConfigUnchanged(existing.Config, effectiveCfg) {
				continue
			}

			// Log any changes.
			c.logCaptureConfigChange(key, existing, effectiveCfg)

			// rebuild or close collector to reflect override changes
			coll, err := c.buildCollector(res, md, effectiveCfg, c.maxCaptureFileSize, c.mongo.collection)
			if err != nil {
				c.logger.Warnw("failed to build collector for capture config", "error", err, "key", key)
				continue
			}
			if existing != nil {
				toClose = append(toClose, existing)
			}
			updates = append(updates, collectorUpdate{md, coll})
		}
	}

	if len(updates) == 0 {
		return
	}

	// Update the collectors map atomically
	c.collectorsMu.Lock()
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
	c.collectorsMu.Unlock()
}

// captureConfigUnchanged returns true when the only fields SetCaptureConfigs can modify —
// CaptureFrequencyHz, Disabled, and Tags — are identical between the existing collector's
// config and the newly computed effective config.
func captureConfigUnchanged(existing, effective datamanager.DataCaptureConfig) bool {
	return existing.CaptureFrequencyHz == effective.CaptureFrequencyHz &&
		existing.Disabled == effective.Disabled &&
		slices.Equal(existing.Tags, effective.Tags)
}

// logCaptureConfigChange logs what changed between the existing collector's config
// and the newly computed effectiveCfg. existing may be nil when creating a new collector.
func (c *Capture) logCaptureConfigChange(key string, existing *collectorAndConfig, effectiveCfg datamanager.DataCaptureConfig) {
	if existing == nil {
		c.logger.Infof("capture config enabling capture for %s: capture_frequency_hz=%f tags=%v",
			key, effectiveCfg.CaptureFrequencyHz, effectiveCfg.Tags)
		return
	}
	prev := existing.Config
	if prev.CaptureFrequencyHz != effectiveCfg.CaptureFrequencyHz {
		c.logger.Infof("capture config changing capture_frequency_hz for %s: %f -> %f",
			key, prev.CaptureFrequencyHz, effectiveCfg.CaptureFrequencyHz)
	}
	if prev.Disabled && !effectiveCfg.Disabled {
		c.logger.Infof("capture config enabling previously disabled collector for %s", key)
	}
	if !slices.Equal(prev.Tags, effectiveCfg.Tags) {
		c.logger.Infof("capture config changing tags for %s: %v -> %v", key, prev.Tags, effectiveCfg.Tags)
	}
}
