package main

import "C"
import (
	"github.com/bitomia/realm/clib/common"
	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/internal/dto"
)

//export PlanLoads
func PlanLoads() *C.char {
	loads := config.GetLoads()
	if len(loads) == 0 {
		return MakeCString(common.ToJsonCString(dto.ErrorResponse{Message: "No loads found"}))
	}
	client := clientPkg.NewClient()
	for _, load := range loads {
		if err := client.PlanLoad(load); err != nil {
			return MakeCString(common.ToJsonCString(err))
		}
	}
	return nil
}
