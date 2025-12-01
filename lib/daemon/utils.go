package main

/*
#include <stdlib.h>
*/
import "C"
import "unsafe"

//export FreeCString
func FreeCString(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func MakeCString(s string) *C.char {
	return C.CString(s)
}
