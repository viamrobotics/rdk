package pointcloud

import (
	"fmt"
)

var pcTypes = map[string]TypeConfig{}

type TypeConfig struct {
	StructureType string
	New func() PointCloud
	NewWithParams func(size int) PointCloud
}

func Register(cfg TypeConfig) {
	_, ok := pcTypes[cfg.StructureType]
	if ok {
		panic(fmt.Errorf("type already registed for [%s]", cfg.StructureType))
	}
	pcTypes[cfg.StructureType] = cfg
}

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
	
