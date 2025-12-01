package dto

import (
	"time"
)

type Image struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type ImagesResponse []Image

type NodeImagesResponse struct {
	Node   string         `json:"node"`
	Images ImagesResponse `json:"images,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type NodeImagesMapResponse []NodeImagesResponse
