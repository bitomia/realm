package internal

const bytesToMB = 1024.0 * 1024.0

func ToMB(bytes float64) float64 {
	return bytes / bytesToMB
}
