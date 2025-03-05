package resource

import (
	pb "go.viam.com/api/robot/v1"
)

type (
	// ModuleModelDiscovery holds the API and Model information of models within a module.
	ModuleModelDiscovery struct {
		ModuleName      string
		API             API
		Model           Model
		FromLocalModule bool
	}
)

// ToProto converts a ModuleModelDiscovery into the equivalent proto message.
func (mm *ModuleModelDiscovery) ToProto() *pb.ModuleModel {
	return &pb.ModuleModel{
		Model: mm.Model.String(), Api: mm.API.String(), ModuleName: mm.ModuleName,
		FromLocalModule: mm.FromLocalModule,
	}
}
