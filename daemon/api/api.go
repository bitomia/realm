package api

import "encoding/json"

// APIResponse represents a standard response format for C exports
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ResponseToJSON converts an API response to JSON string
func ResponseToJSON(success bool, data any, errMsg string) string {
	resp := APIResponse{
		Success: success,
		Data:    data,
		Error:   errMsg,
	}
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		// Fallback error response
		return `{"success":false,"error":"failed to marshal response"}`
	}
	return string(jsonBytes)
}
