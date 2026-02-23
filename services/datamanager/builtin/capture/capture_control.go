package capture

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

// captureConfigKey returns the lookup key for a per-resource capture config map.
func captureConfigKey(resourceShortName, method string) string {
	return fmt.Sprintf("%s/%s", resourceShortName, method)
}

// captureConfigsEqual returns true when two per-resource config maps are semantically equal.
func captureConfigsEqual(a, b map[string]datamanager.CaptureConfigReading) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if (av.CaptureFrequencyHz == nil) != (bv.CaptureFrequencyHz == nil) {
			return false
		}
		if av.CaptureFrequencyHz != nil && *av.CaptureFrequencyHz != *bv.CaptureFrequencyHz {
			return false
		}
		// nil means "no tag override" while []string{} means "override to empty" — treat them differently.
		if (av.Tags == nil) != (bv.Tags == nil) {
			return false
		}
		if av.Tags != nil && !slices.Equal(av.Tags, bv.Tags) {
			return false
		}
	}
	return true
}

// fmtFloat32Ptr formats a *float32 for logging.
func fmtFloat32Ptr(f *float32) string {
	if f == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%f", *f)
}

// SetCaptureConfig applies dynamic per-resource capture configs without triggering a full Reconfigure.
// Only collectors whose effective config (base + override) has changed are updated.
// Passing nil or an empty map reverts all collectors to their base machine configs.
// configs is keyed by "resourceShortName/method" (e.g. "camera-1/GetImages").
func (c *Capture) SetCaptureConfig(ctx context.Context, configs map[string]datamanager.CaptureConfigReading) {
	if captureConfigsEqual(c.currentCaptureConfig, configs) {
		return
	}

	// affectedKeys is the union of previously-overridden and newly-overridden resource/method pairs.
	// These are the only collectors whose effective config may have changed.
	affectedKeys := make(map[string]struct{}, len(c.currentCaptureConfig)+len(configs))
	for k := range c.currentCaptureConfig {
		affectedKeys[k] = struct{}{}
	}
	for k := range configs {
		affectedKeys[k] = struct{}{}
	}
	c.logger.Infof("SetCaptureConfig: applying %d configs (affecting %d keys)", len(configs), len(affectedKeys))
	c.currentCaptureConfig = configs

	type collectorUpdate struct {
		md  collectorMetadata
		cac *collectorAndConfig // nil means remove
	}
	var toClose []*collectorAndConfig
	var updates []collectorUpdate

	for res, cfgs := range c.baseCollectorConfigs {
		for _, cfg := range cfgs {
			key := captureConfigKey(cfg.Name.ShortName(), cfg.Method)
			if _, affected := affectedKeys[key]; !affected {
				continue
			}

			// Apply service-level settings.
			cfg.Tags = c.baseCaptureConfig.Tags
			if c.baseCaptureConfig.CaptureDisabled {
				cfg.Disabled = true
			}

			// Apply per-resource config if one exists for this key, otherwise revert to base.
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
				// Any config re-enables if effective frequency is positive —
				// even a tags-only config. Zero freq still disables.
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
			} else {
				c.logger.Infof("reverting %s to base config", key)
			}

			md := newCollectorMetadata(cfg)
			// Safe to read c.collectors without collectorsMu: SetCaptureConfig is always called
			// under b.mu, and all writers of c.collectors also hold b.mu.
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

			// buildCollector skips queue/buffer/additional-params validation: those fields
			// are unchanged from the base config that was already validated during Reconfigure.
			cac, err := c.buildCollector(res, md, cfg, c.baseCaptureConfig, c.mongo.collection)
			if err != nil {
				c.logger.Warnw("failed to build collector for capture config", "error", err, "key", key)
				continue
			}
			if existing != nil {
				toClose = append(toClose, existing)
			}
			updates = append(updates, collectorUpdate{md, cac})
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

// buildCollector constructs and starts a new collector for res/md.
// It does not validate queue size, buffer size, or additional params — those are
// validated once by initializeOrUpdateCollector on the Reconfigure path.
// The control path (SetCaptureConfig) calls this directly since the base config was already validated.
func (c *Capture) buildCollector(
	res resource.Resource,
	md collectorMetadata,
	collectorConfig datamanager.DataCaptureConfig,
	config Config,
	collection *mongo.Collection,
) (*collectorAndConfig, error) {
	methodParams, err := protoutils.ConvertMapToProtoAny(collectorConfig.AdditionalParams)
	if err != nil {
		return nil, err
	}

	// Get collector constructor for the component API and method.
	collectorConstructor := data.CollectorLookup(md.MethodMetadata)
	if collectorConstructor == nil {
		return nil, errors.Errorf("failed to find collector constructor for %s", md.MethodMetadata)
	}

	targetDir := targetDir(config.CaptureDir, collectorConfig)
	// Create a collector for this resource and method.
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, errors.Wrapf(err, "failed to create target directory %s with 700 file permissions", targetDir)
	}
	// Build metadata.
	captureMetadata, dataType := data.BuildCaptureMetadata(
		collectorConfig.Name.API,
		collectorConfig.Name.ShortName(),
		collectorConfig.Method,
		collectorConfig.AdditionalParams,
		methodParams,
		collectorConfig.Tags,
	)
	// Parameters to initialize collector.
	queueSize := defaultIfZeroVal(collectorConfig.CaptureQueueSize, defaultCaptureQueueSize)
	bufferSize := defaultIfZeroVal(collectorConfig.CaptureBufferSize, defaultCaptureBufferSize)
	collector, err := collectorConstructor(res, data.CollectorParams{
		MongoCollection: collection,
		DataType:        dataType,
		ComponentName:   collectorConfig.Name.ShortName(),
		ComponentType:   collectorConfig.Name.API.String(),
		MethodName:      collectorConfig.Method,
		Interval:        data.GetDurationFromHz(collectorConfig.CaptureFrequencyHz),
		MethodParams:    methodParams,
		Target:          data.NewCaptureBuffer(targetDir, captureMetadata, config.MaximumCaptureFileSizeBytes),
		// Set queue size to defaultCaptureQueueSize if it was not set in the config.
		QueueSize:  queueSize,
		BufferSize: bufferSize,
		Logger:     c.logger,
		Clock:      c.clk,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "constructor for collector %s failed with config: %s",
			md, collectorConfigDescription(collectorConfig, targetDir, config.MaximumCaptureFileSizeBytes, queueSize, bufferSize))
	}

	c.logger.Infof("collector initialized; collector: %s, config: %s",
		md, collectorConfigDescription(collectorConfig, targetDir, config.MaximumCaptureFileSizeBytes, queueSize, bufferSize))
	collector.Collect()

	return &collectorAndConfig{res, collector, collectorConfig}, nil
}
