package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/lib/common"
)

//export GetNodeImagesMap
func GetNodeImagesMap() *C.char {
	client := clientPkg.NewClient()

	nodeImagesMap, err := client.GetAllImages()
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	return MakeCString(common.ToJsonCString(nodeImagesMap))
}
