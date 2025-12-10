package common

import "C"
import (
	"encoding/json"

	"github.com/bitomia/realm/common/dto"
)

func toJsonError(e error) string {
	errorJson, _ := json.Marshal(dto.ErrorResponse{
		Message: e.Error(),
	})
	return string(errorJson)
}

func ToJsonCString(o any) string {
	e, isError := o.(error)
	if isError {
		return toJsonError(e)
	}

	json, err := json.Marshal(o)
	if err != nil {
		return toJsonError(e)
	}
	return string(json)
}
