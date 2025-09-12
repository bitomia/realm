package utils

import (
	"fmt"
	"net/http"
)

func HttpError(w http.ResponseWriter, statusCode int, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	http.Error(w, msg, statusCode)
}
