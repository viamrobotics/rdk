package resource

import (
	pb "go.viam.com/api/robot/v1"
)

type (
	// ModuleModel holds the API and Model information of models within a module.
	ModuleModel struct {
		ModuleName      string
		API             API
		Model           Model
		FromLocalModule bool
	}
)

// ToProto converts a ModuleModel into the equivalent proto message.
func (mm *ModuleModel) ToProto() *pb.ModuleModel {
	return &pb.ModuleModel{
		Model: mm.Model.String(), Api: mm.API.String(), ModuleName: mm.ModuleName,
		FromLocalModule: mm.FromLocalModule,
	}
}
