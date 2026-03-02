package capture

import (
	"context"
	"fmt"

	"go.viam.com/rdk/services/datamanager"
)

// captureConfigKey returns the lookup key for a per-resource capture config map.
func captureConfigKey(resourceString, method string) string {
	return fmt.Sprintf("%s/%s", resourceString, method)
}

// fmtFloat32Ptr formats a *float32 for logging.
func fmtFloat32Ptr(f *float32) string {
	if f == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%f", *f)
}

// SetCaptureConfigs applies dynamic per-resource capture configs without triggering a full Reconfigure.
// Only collectors whose effective config (base + override) has changed are updated.
// Passing nil or an empty map reverts all collectors to their base machine configs.
// configs is keyed by "resourceName/method" (e.g. "camera-1/GetImages").
func (c *Capture) SetCaptureConfigs(ctx context.Context, configs map[string]datamanager.CaptureConfigReading) {
	type collectorUpdate struct {
		md  collectorMetadata
		cac *collectorAndConfig // nil means remove
	}
	var toClose []*collectorAndConfig
	var updates []collectorUpdate

	for res, cfgs := range c.baseCollectorConfigs {
		for _, cfg := range cfgs {
			key := captureConfigKey(cfg.Name.ShortName(), cfg.Method)

			// Apply service-level tags.
			cfg.Tags = c.baseTags

			// Apply per-resource override if present, otherwise use base config as-is.
			if config, ok := configs[key]; ok {
				c.logger.Infof("applying capture config for %s: capture_frequency_hz=%s tags=%v",
					key, fmtFloat32Ptr(config.CaptureFrequencyHz), config.Tags)
				wasDisabled := cfg.Disabled
				if config.CaptureFrequencyHz != nil {
					oldFreq := cfg.CaptureFrequencyHz
					cfg.CaptureFrequencyHz = *config.CaptureFrequencyHz
					if cfg.CaptureFrequencyHz != oldFreq {
						c.logger.Infof("capture config changing capture_frequency_hz for %s: %f -> %f",
							key, oldFreq, cfg.CaptureFrequencyHz)
					}
				}

				if cfg.CaptureFrequencyHz > 0 {
					cfg.Disabled = false
				}
				if wasDisabled && !cfg.Disabled {
					c.logger.Infof("capture config enabling previously disabled collector for %s", key)
				}
				if config.Tags != nil {
					c.logger.Infof("capture config changing tags for %s: %v -> %v", key, cfg.Tags, config.Tags)
					cfg.Tags = config.Tags
				}
			}

			md := newCollectorMetadata(cfg)
			existing := c.collectors[md]

			if cfg.Disabled || cfg.CaptureFrequencyHz <= 0 {
				if existing != nil {
					c.logger.Infof("capture config disabling collector for %s", key)
					toClose = append(toClose, existing)
					updates = append(updates, collectorUpdate{md, nil})
				}
				continue
			}

			// Skip if the effective config is unchanged.
			if existing != nil && existing.Config.Equals(&cfg) && res == existing.Resource {
				continue
			}

			coll, err := c.buildCollector(res, md, cfg, c.maxCaptureFileSize, c.mongo.collection)
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

	// Update the collectors map atomically, then close replaced collectors.
	c.collectorsMu.Lock()
	for _, u := range updates {
		if u.cac != nil {
			c.collectors[u.md] = u.cac
		} else {
			delete(c.collectors, u.md)
		}
	}
	c.collectorsMu.Unlock()

	for _, old := range toClose {
		old.Collector.Close()
	}
}
