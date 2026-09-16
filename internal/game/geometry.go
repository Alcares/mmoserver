package game

import "math"

type Vec2f struct {
	X float64
	Y float64
}

func CompactCoordinates(x, y int32) uint64 {
	return (uint64(uint32(x)) << 32) | uint64(uint32(y))
}

// ellipsePoints assign visually evenly distributed position around central point
// in an elliptical orbit
func ellipsePoints(cx, cy, a, b float64, n int) []Vec2f {
	pts := make([]Vec2f, n)
	for i := range n {
		theta := 2 * math.Pi * float64(i) / float64(n)
		pts[i] = Vec2f{
			X: cx + a*math.Cos(theta),
			Y: cy + b*math.Sin(theta),
		}
	}
	return pts
}

func EuclideanDistance(p1, p2 Vec2f) float64 {
	return math.Sqrt(math.Pow(math.Abs(p1.X-p2.X), 2) + math.Pow(math.Abs(p1.Y-p2.Y), 2))
}
