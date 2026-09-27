package geometry

import (
	"math"
	"math/rand"
)

type Vec2f struct {
	X float64
	Y float64
}

func CompactCoordinates(x, y int32) uint64 {
	return (uint64(uint32(x)) << 32) | uint64(uint32(y))
}

// EllipsePoints assign visually evenly distributed position around central point
// in an elliptical orbit
func EllipsePoints(centerX, centerY, radiusX, radiusY float64, count int) []Vec2f {
	points := make([]Vec2f, count)
	for i := range count {
		theta := 2 * math.Pi * float64(i) / float64(count)
		points[i] = Vec2f{
			X: centerX + radiusX*math.Cos(theta),
			Y: centerY + radiusY*math.Sin(theta),
		}
	}
	return points
}

func EuclideanDistance(from, to Vec2f) float64 {
	return math.Hypot(from.X-to.X, from.Y-to.Y)
}

// Normalize returns v scaled to length 1. The zero vector has no direction and comes back as is.
func Normalize(v Vec2f) Vec2f {
	length := math.Hypot(v.X, v.Y)
	if length == 0 {
		return v
	}
	return Vec2f{X: v.X / length, Y: v.Y / length}
}

// RandomPointInDisc returns a point uniformly distributed over the disc of radius around center.
func RandomPointInDisc(rng *rand.Rand, center Vec2f, radius float64) Vec2f {
	// sqrt keeps the density uniform over the disc instead of bunching at the centre
	r := radius * math.Sqrt(rng.Float64())
	theta := 2 * math.Pi * rng.Float64()
	return Vec2f{X: center.X + r*math.Cos(theta), Y: center.Y + r*math.Sin(theta)}
}

// ProjectOntoCircle returns the point on the circle around center that is closest to point: in the
// same direction from center as point, exactly radius away. It moves point outward or inward alike.
func ProjectOntoCircle(center, point Vec2f, radius float64) Vec2f {
	direction := Normalize(Vec2f{X: point.X - center.X, Y: point.Y - center.Y})
	if direction == (Vec2f{}) { // exactly on the centre: no direction to go in, so pick north
		direction = Vec2f{X: 0, Y: -1}
	}
	return Vec2f{
		X: center.X + direction.X*radius,
		Y: center.Y + direction.Y*radius,
	}
}

// RayCircle returns how far along the unit direction from origin the ray first touches the circle.
// A ray starting on or inside the circle hits at 0 if it heads further in, and misses if it heads out.
func RayCircle(origin, direction, center Vec2f, radius float64) (float64, bool) {
	mx, my := origin.X-center.X, origin.Y-center.Y
	b := mx*direction.X + my*direction.Y
	if b >= 0 {
		return 0, false // heading away from the centre, or passing it tangentially
	}
	c := mx*mx + my*my - radius*radius
	if c <= 0 {
		return 0, true
	}
	disc := b*b - c
	if disc < 0 {
		return 0, false
	}
	return -b - math.Sqrt(disc), true
}

// EdgeDistance returns how far along the unit direction from origin the ray leaves the
// [0, size] square.
func EdgeDistance(origin, direction Vec2f, size float64) float64 {
	dist := math.Inf(1)
	for _, axis := range [2][2]float64{{origin.X, direction.X}, {origin.Y, direction.Y}} {
		pos, dir := axis[0], axis[1]
		if dir > 0 {
			dist = min(dist, (size-pos)/dir)
		} else if dir < 0 {
			dist = min(dist, -pos/dir)
		}
	}
	return dist
}
