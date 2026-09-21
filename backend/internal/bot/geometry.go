package bot

import "math"

func normalize(x float32, divisor float32) float32 {
	return x / divisor
}

func distance(x float32, y float32) float32 {
	return float32(math.Sqrt(float64(x*x + y*y)))
}
