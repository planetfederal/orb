package planar

import (
	"bytes"
	"fmt"
	"math"
)

// Polygon is a closed area. The first LineString is the outer ring.
// The others are the holes. Each LineString is expected to be closed
// ie. the first point matches the last.
type Polygon []LineString

// NewPolygon creates a new Polygon.
func NewPolygon() Polygon {
	return Polygon{}
}

// DistanceFrom will return the distance from the point to
// the polygon. Returns 0 if the point is within the polygon.
func (p Polygon) DistanceFrom(point Point) float64 {
	ring := Polygon{p[0]}
	if !ring.Contains(point) {
		return p[0].DistanceFrom(point)
	}

	// since we're within, check the holes
	for i := 1; i < len(p); i++ {
		hole := Polygon{p[i]}
		if hole.Contains(point) {
			return p[i].DistanceFrom(point)
		}
	}

	// within the polygon, but not within any of the holes.
	return 0
}

// Centroid computes the area based centroid of the polygon.
// The algorithm removes the contribution of the holes.
func (p Polygon) Centroid() Point {
	point, _ := p.CentroidArea()
	return point
}

// CentroidArea computes the centroid and returns the area.
// If you need both this is faster since we need to area to compute the centroid.
func (p Polygon) CentroidArea() (Point, float64) {
	centroid, area := p[0].ringCentroid()

	holeArea := 0.0
	holeCentroid := Point{}
	for i := 1; i < len(p); i++ {
		ring := p[i]

		hc, ha := ring.ringCentroid()
		holeArea += ha
		holeCentroid[0] += hc[0] * ha
		holeCentroid[1] += hc[1] * ha
	}

	centroid[0] = (area*centroid[0] - holeArea*holeCentroid[0]) / (area - holeArea)
	centroid[1] = (area*centroid[1] - holeArea*holeCentroid[1]) / (area - holeArea)

	return centroid, area - holeArea
}

func (ls LineString) ringCentroid() (Point, float64) {
	ring := ls
	centroid := Point{}

	area := 0.0

	// implicitly move everything to near the origin to help with roundoff
	offsetX := ring[0][0]
	offsetY := ring[0][1]
	for i := 1; i < len(ring)-1; i++ {
		a := (ring[i][0]-offsetX)*(ring[i+1][1]-offsetY) -
			(ring[i+1][0]-offsetX)*(ring[i][1]-offsetY)
		area += a

		centroid[0] += (ring[i][0] + ring[i+1][0] - 2*offsetX) * a
		centroid[1] += (ring[i][1] + ring[i+1][1] - 2*offsetY) * a
	}

	// no need to deal with first and last vertex since we "moved"
	// that point the origin (multiply by 0 == 0)

	area /= 2
	centroid[0] /= 6 * area
	centroid[1] /= 6 * area

	centroid[0] += offsetX
	centroid[1] += offsetY

	return centroid, area
}

// Contains checks if the point is within the polygon.
// Points on the boundary are considered in.
func (p Polygon) Contains(point Point) bool {
	c := lineStringContains(p[0], point)
	if !c {
		return false
	}

	for i := 1; i < len(p); i++ {
		if lineStringContains(p[i], point) {
			return false
		}
	}

	return true
}

// Area computes the positive area of the polygon minus the area
// of the holes.
func (p Polygon) Area() float64 {
	area := lineStringArea(p[0])

	for i := 1; i < len(p); i++ {
		// minus holes
		area -= lineStringArea(p[i])
	}

	return area
}

// Bound returns a bound around the polygon.
func (p Polygon) Bound() Rect {
	return p[0].Bound()
}

// WKT returns the polygon in WKT format, eg. POlYGON((0 0,1 0,1 1,0 0))
// For empty polygons the result will be 'EMPTY'.
func (p Polygon) WKT() string {
	if len(p) == 0 {
		return "EMPTY"
	}

	buff := bytes.NewBuffer(nil)
	fmt.Fprintf(buff, "POLYGON(")
	wktPoints(buff, p[0])

	for i := 1; i < len(p); i++ {
		buff.Write([]byte(","))
		wktPoints(buff, p[i])
	}

	buff.Write([]byte(")"))
	return buff.String()
}

// Equal compares two polygons. Returns true if lengths are the same
// and all points are Equal.
func (p Polygon) Equal(polygon Polygon) bool {
	return MultiLineString(p).Equal(MultiLineString(polygon))
}

// Clone returns a new deep copy of the polygon.
// All of the rings are also cloned.
func (p Polygon) Clone() Polygon {
	return Polygon(MultiLineString(p).Clone())
}

func lineStringArea(ls LineString) float64 {
	if len(ls) == 0 {
		return 0
	}

	area := 0.0
	for i := 1; i < len(ls)-1; i++ {
		area += ls[i][0] * (ls[i+1][1] - ls[i-1][1])
	}

	// for i == N-1
	last := len(ls) - 1
	area += ls[last][0] * (ls[0][1] - ls[last-1][1])

	// for i == N
	area += ls[0][0] * (ls[1][1] - ls[last][1])

	return math.Abs(area / 2.0)
}

func lineStringContains(ls LineString, point Point) bool {
	if !ls.Bound().Contains(point) {
		return false
	}

	c, on := rayIntersect(point, ls[0], ls[len(ls)-1])
	if on {
		return true
	}

	for i := 0; i < len(ls)-1; i++ {
		inter, on := rayIntersect(point, ls[i], ls[i+1])
		if on {
			return true
		}

		if inter {
			c = !c
		}
	}

	return c
}

// Original implementation: http://rosettacode.org/wiki/Ray-casting_algorithm#Go
func rayIntersect(p, s, e Point) (intersects, on bool) {
	if s[0] > e[0] {
		s, e = e, s
	}

	if p[0] == s[0] {
		if p[1] == s[1] {
			// p == start
			return false, true
		} else if s[0] == e[0] {
			// vertical segment (s -> e)
			// return true if within the line, check to see if start or end is greater.
			if s[1] > e[1] && s[1] >= p[1] && p[1] >= e[1] {
				return false, true
			}

			if e[1] > s[1] && e[1] >= p[1] && p[1] >= s[1] {
				return false, true
			}
		}

		// Move the y coordinate to deal with degenerate case
		p[0] = math.Nextafter(p[0], math.Inf(1))
	} else if p[0] == e[0] {
		if p[1] == e[1] {
			// matching the end point
			return false, true
		}

		p[0] = math.Nextafter(p[0], math.Inf(1))
	}

	if p[0] < s[0] || p[0] > e[0] {
		return false, false
	}

	if s[1] > e[1] {
		if p[1] > s[1] {
			return false, false
		} else if p[1] < e[1] {
			return true, false
		}
	} else {
		if p[1] > e[1] {
			return false, false
		} else if p[1] < s[1] {
			return true, false
		}
	}

	rs := (p[1] - s[1]) / (p[0] - s[0])
	ds := (e[1] - s[1]) / (e[0] - s[0])

	if rs == ds {
		return false, true
	}

	return rs <= ds, false
}