package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

type splice struct {
	tool     int
	position float64
	length   float64
}

type Generator struct {
	config *Config
	out    io.Writer

	totalExtruded float64
	currentTool   int
	currentSplice float64
	currentStart  float64
	sinceLastPing float64
	splices       []splice
	pings         []float64
}

func (g *Generator) extrude(f float64) float64 {
	g.totalExtruded += f
	g.currentSplice += f
	g.sinceLastPing += f

	return f
}

func (g *Generator) writeOmegaHeader(out io.Writer) {
	fmt.Fprintf(out, "O21 D%04X ; msf version 2.0 (20 = 0x14)\n", 20)
	fmt.Fprintf(out, "O22 Dcafebabefeedbeef\n") // TODO: take from config
	fmt.Fprintf(out, "O23 D0001 ; unused\n")
	fmt.Fprintf(out, "O24 D0000 ; unused\n")
	fmt.Fprintf(out, "O25 D1FFFFFFWhite_PLA D10F80FFDodgerBlue_PLA D1E8D89AKhaki_PLA D1000000Black_PLA ; inputs: filament type + hex color + color_material\n") // TODO: take from config
	fmt.Fprintf(out, "O26 D%04X ; number of splices\n", len(g.splices))
	fmt.Fprintf(out, "O27 D%04X ; number of pings\n", len(g.pings))
	fmt.Fprintf(out, "O28 D0001 ; number of splice algorithms\n") // TODO: dunno
	fmt.Fprintf(out, "O29 D0000 ; number of hotswaps\n")
	for _, splice := range g.splices {
		fmt.Fprintf(out, "O30 D%d %s\n", splice.tool, floatToHex(float32(splice.position+splice.length)))
	}
	// O31 D[tool] D[ping location] ; accessory mode only
	fmt.Fprintf(out, "O32 D11 D0000 D0000 D0000 ; splice algorithm table\n") // TODO: dunno
	fmt.Fprintf(out, "O1 Dbleedsquares D%08X\n", int(g.totalExtruded))

	fmt.Fprintln(out, "\n")
	for _, splice := range g.splices {
		fmt.Fprintf(out, "; Tool: %d Location: %.2f length %.2f  ends %.2f (%s)\n", splice.tool, splice.position, splice.length, splice.position+splice.length, floatToHex(float32(splice.position+splice.length)))
	}
	fmt.Fprintln(out, "\n")

	fmt.Fprintln(out, "M0")
	fmt.Fprintln(out, "T0")
	fmt.Fprintln(out, "M107")
}

func (g *Generator) writeStartGCode() {

	fmt.Fprintln(g.out, "\n; --- BEGIN start_gcode ---\n")
	lines := strings.Split(g.config.StartGCode(), "\\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ";P2PP") {
			continue
		}
		if strings.ContainsAny(line, "[]") {
			line = strings.Replace(line, "[first_layer_bed_temperature]", g.config.FirstLayerBedTemp()[0], 1)
			line = strings.Replace(line, "[first_layer_temperature]", g.config.FirstLayerTemp()[0], 1)
		}

		fmt.Fprintln(g.out, line)
	}
	fmt.Fprintln(g.out, "\n; --- END start_gcode ---\n")

	//TODO: hardcoding this for now, need to parse extrusion done in the start code
	g.extrude(21.5) // priming stroke
	// is this the right way to add the splice offset?
	g.extrude(g.config.SpliceOffset())
}

func (g *Generator) writeEndGCode() {
	// final toolchange to save the last splice
	g.extrude(g.config.ExtraEndFilament()) // add the tail to the final splice
	g.toolchange(5)

	fmt.Fprintln(g.out, "\n; --- BEGIN end_gcode ---\n")
	lines := strings.Split(g.config.EndGCode(), "\\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ";P2PP") {
			continue
		}

		fmt.Fprintln(g.out, line)
	}
	fmt.Fprintln(g.out, "\n; --- END end_gcode ---\n")
}

// volume in mm3
// returns length extruded in mm
func (g *Generator) purgeSquare(xIdx, yIdx int, volume float64) float64 {
	atX := float64(xIdx)*70.0 + 10.0
	atY := 290.0 - (float64(yIdx)*70.0 + 10.0)

	lineVolume := 5.0 // 5mm3 per line
	ystep := g.config.ExtrusionWidth()

	lineXSection := g.config.ExtrusionWidth() * g.config.LayerHeight()

	// a line includes the width plus one y move down
	// lineVolume / lineXSection gives us the length to move, subtract ystep to get width
	width := (lineVolume / lineXSection) - ystep

	//fmt.Printf("a %.2f x %.2f line has a cross-section of %.2fmm2, and that line %.2fmm long has a volume of %.3fmm3\n",
	//	g.config.LayerHeight(), g.config.ExtrusionWidth(), lineXSection, (width + ystep), lineVolume)

	filamentXsection := math.Pi * math.Pow(g.config.FilamentDiameter()[0]/2, 2) // tool 0
	// volume is cross-section * length, so length is volume / cross-section

	// need the partial volume of the X move and Y step
	YstepVolume := ystep * lineXSection
	Xvolume := lineVolume - YstepVolume

	XextrudeLength := Xvolume / filamentXsection
	YextrudeLength := YstepVolume / filamentXsection

	//fmt.Printf("X moves will be %.2f long and %.3fmm3, consuming %.3fmm filament\n", width, Xvolume, XextrudeLength)
	//fmt.Printf("Ystep will be %.2f long and %.3fmm3, consuming %.3fmm filament\n", ystep, YstepVolume, YextrudeLength)

	fmt.Fprintf(g.out, "\n; --- purge block at %.2f, %.2f layer height %.2f ---\n\n", atX, atY, g.config.LayerHeight())

	Y := atY
	E := 0.0
	// draw the outline first
	boxHeight := ((volume / lineVolume) + 1) * g.config.ExtrusionWidth()
	boxWidth := width + g.config.ExtrusionWidth()

	fmt.Fprintf(g.out, "G0 X%.3f Y%.3f F9000\n", atX+boxWidth, atY)
	fmt.Fprintf(g.out, "G1 Z%.3f F600\n", g.config.LayerHeight())
	fmt.Fprintf(g.out, "M82\nG92 E0\n")
	E += g.extrude((boxHeight * lineXSection) / filamentXsection)
	fmt.Fprintf(g.out, "G1 Y%.3f E%.4f F4000\n", atY-boxHeight, E)
	E += g.extrude((boxWidth * lineXSection) / filamentXsection)
	fmt.Fprintf(g.out, "G1 X%.3f E%.4f\n", atX, E)
	E += g.extrude((boxHeight * lineXSection) / filamentXsection)
	fmt.Fprintf(g.out, "G1 Y%.3f E%.4f F4000\n", atY, E)

	volume -= E * filamentXsection
	//fmt.Printf("used up %.3fmm %.3fmm3 in the box\n", E, E*filamentXsection)

	for i := 0.0; i < volume; i += lineVolume * 2 {
		if g.sinceLastPing > g.config.LinearPing() {
			// do ping to it
			g.ping()
		}
		E += g.extrude(XextrudeLength)
		fmt.Fprintf(g.out, "G1 X%.3f E%.4f F4000\n", atX+width+g.config.ExtrusionWidth()/2, E)
		E += g.extrude(YextrudeLength)
		Y -= ystep
		fmt.Fprintf(g.out, "G1 Y%.3f E%.4f\n", Y, E)
		E += g.extrude(XextrudeLength)
		fmt.Fprintf(g.out, "G1 X%.3f E%.4f\n", atX+g.config.ExtrusionWidth()/2, E)
		E += g.extrude(YextrudeLength)
		Y -= ystep
		fmt.Fprintf(g.out, "G1 Y%.3f E%.4f\n", Y, E)
	}
	fmt.Fprintf(g.out, "G1 Z%.3f F600\n", g.config.LayerHeight()+0.5)
	fmt.Fprintln(g.out, "\n; --- end purge block ---\n")
	return E
}

func (g *Generator) toolchange(newTool int) {
	g.splices = append(g.splices, splice{g.currentTool, g.currentStart, g.currentSplice})
	g.currentTool = newTool
	g.currentSplice = 0.0
	g.currentStart = g.totalExtruded
}

func (g *Generator) ping() {
	g.pings = append(g.pings, g.totalExtruded)
	g.sinceLastPing = 0.0
	fmt.Fprintf(g.out, "; -- ping! -- \n")
	fmt.Fprintf(g.out, "G4 S0\n")
	fmt.Fprintf(g.out, "O31 %s\n", floatToHex(float32(g.totalExtruded)))
	fmt.Fprintf(g.out, "; -- /ping -- \n")
}

func main() {
	gen := &Generator{}

	outFile, err := os.Create("bleed_test.gcode")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer outFile.Close()

	gen.config, err = loadConfig("head.gcode")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	buf := new(bytes.Buffer)
	gen.out = buf

	// set initial tool
	gen.currentTool = 0

	gen.writeStartGCode()
	// start gcode consumes some filament
	// it'd be nice to parse it and count it up
	// but for now, it looks like 21.5mm in the priming move

	/*

			x***
		  *x**
			**x*
			***x

			1->2
			2->3
			3->4
			4->1
			1->3
			3->2
			2->4
			4->3
			3->1
			1->4
			4->2
			2->1
	*/

	// make a babby square of tool 1 at 0,0
	gen.purgeSquare(0, 0, 200)

	// 1->2 at 1,0
	gen.toolchange(1) // go to tool 2
	gen.purgeSquare(1, 0, 500)

	// 2->3 at 2,1
	gen.toolchange(2) // go to tool 3
	gen.purgeSquare(2, 1, 500)

	// 3->4 at 3,2
	gen.toolchange(3) // go to tool 4
	gen.purgeSquare(3, 2, 500)

	// 4->1 at 0,3
	gen.toolchange(0) // go to tool 1
	gen.purgeSquare(0, 3, 500)

	// 1->3 at 2,0
	gen.toolchange(2) // go to tool 3
	gen.purgeSquare(2, 0, 500)

	// 3->2 at 1,2
	gen.toolchange(1) // go to tool 2
	gen.purgeSquare(1, 2, 500)

	// 2->4 at 3,1
	gen.toolchange(3) // go to tool 4
	gen.purgeSquare(3, 1, 500)

	// 4->3 at 2,3
	gen.toolchange(2) // go to tool 3
	gen.purgeSquare(2, 3, 500)

	// 3->1 at 0,2
	gen.toolchange(0) // go to tool 1
	gen.purgeSquare(0, 2, 500)

	// 1->4 at 3,0
	gen.toolchange(3) // go to tool 4
	gen.purgeSquare(3, 0, 500)

	// 4->2 at 1,3
	gen.toolchange(1) // go to tool 2
	gen.purgeSquare(1, 3, 500)

	// 2->1 at 0,1
	gen.toolchange(0) // go to tool 1
	gen.purgeSquare(0, 1, 500)

	gen.writeEndGCode()

	gen.writeOmegaHeader(outFile)
	buf.WriteTo(outFile)

	for _, f := range []float32{134.29, 302.96, 1528.69, 2201.30} {
		fmt.Printf("%.2f = %s\n", f, floatToHex(f))
	}
}
