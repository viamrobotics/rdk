package pointcloud

import (
	"fmt"
)

var pcTypes = map[string]TypeConfig{}

// TypeConfig a type of pointcloud.
type TypeConfig struct {
	StructureType string
	NewWithParams func(size int) PointCloud
}

// Register a point cloud type.
func Register(cfg TypeConfig) {
	_, ok := pcTypes[cfg.StructureType]
	if ok {
		panic(fmt.Errorf("type already registered for [%s]", cfg.StructureType))
	}
	pcTypes[cfg.StructureType] = cfg
}

// Find a pointcloud type.
func Find(pcStructureType string) (TypeConfig, error) {
	if pcStructureType == "" {
		pcStructureType = BasicType
	}

	cfg, ok := pcTypes[pcStructureType]
	if !ok {
		return TypeConfig{}, fmt.Errorf("no point cloud type registered for [%s]", pcStructureType)
	}
	return cfg, nil
}
