package dto

type FileDataResponse struct {
	Data   string `json:"data"`
	EndPos int64  `json:"end_pos"`
}
