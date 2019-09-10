package main

import "image"

type square struct {
	bounds       image.Rectangle
	fromExtruder int
	toExtruder   int
}

func (g *Generator) layoutSquares() {
	g.squares = make([]square, 0)
	g.colormarkers = make([]square, 0)

	printableArea := g.config.BedDimensions().Inset(g.config.Margin())

	pad := g.config.Padding()

	// it's a 4x4 grid, so we need to divide the area by four, but also
	// need to leave padding between the squares, so to find the width
	// of a square it's (area - 3*padding)/4
	width := (printableArea.Dx() - 3*pad) / 4
	height := (printableArea.Dy() - 3*pad) / 4

	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if x == y {
				// for the identity squares, print a half-size square in that color
				bottomleft := image.Pt(x*(width+pad)+width/4, y*(height+pad)+height/4).Add(printableArea.Min)
				topright := bottomleft.Add(image.Pt(width/2, height/2))
				g.colormarkers = append(g.colormarkers, square{image.Rectangle{bottomleft, topright}, x, x})
				continue
			}
			bottomleft := image.Pt(x*(width+pad), y*(height+pad)).Add(printableArea.Min)
			topright := bottomleft.Add(image.Pt(width, height))

			// i'm inverting y for the extruder number because the printer is origin bottom left
			// so bottom row is row 0, but we want top row to be extruder 0
			g.squares = append(g.squares, square{image.Rectangle{bottomleft, topright}, 3 - y, 3 - x})
		}
	}
}
