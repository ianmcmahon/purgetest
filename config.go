package main

import (
	"bufio"
	"fmt"
	"image"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	rawvals map[string]string
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{
		rawvals: make(map[string]string, 0),
	}

	p2ppRE := regexp.MustCompile(`;\s*P2PP\s+(\S+)\s*=\s*(\S+)`)

	scanner := bufio.NewScanner(file)
	complete := false
	for scanner.Scan() {
		line := scanner.Text()

		if !complete {
			// need to catch the P2PP configs at the top
			p2ppMatches := p2ppRE.FindStringSubmatch(line)
			if len(p2ppMatches) == 3 {
				config.rawvals[fmt.Sprintf("P2PP_%s", p2ppMatches[1])] = p2ppMatches[2]
			}

			if strings.HasPrefix(line, "; estimated printing time") {
				complete = true
			}
			continue
		}
		if !strings.HasPrefix(line, "; ") {
			continue
		}
		toks := strings.Split(strings.TrimPrefix(line, "; "), " = ")
		config.rawvals[toks[0]] = toks[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return config, nil
}

func (c *Config) AsFloat(key string) float64 {
	v, err := strconv.ParseFloat(c.rawvals[key], 64)
	if err != nil {
		panic(err)
	}
	return v
}

func (c *Config) AsFloatArray(key string) []float64 {
	toks := strings.Split(c.rawvals[key], ",")
	vals := make([]float64, len(toks))
	for i, tok := range toks {
		v, err := strconv.ParseFloat(tok, 64)
		if err != nil {
			panic(err)
		}
		vals[i] = v
	}
	return vals
}

func (c *Config) AsStringArray(key string) []string {
	return strings.Split(c.rawvals[key], ",")
}

func (c *Config) StartGCode() string {
	return c.rawvals["start_gcode"]
}

func (c *Config) EndGCode() string {
	return c.rawvals["end_gcode"]
}

func (c *Config) ExtrusionWidth() float64 {
	return c.AsFloat("extrusion_width")
}

func (c *Config) LayerHeight() float64 {
	return c.AsFloat("layer_height")
}

func (c *Config) FilamentDiameter() []float64 {
	return c.AsFloatArray("filament_diameter")
}

func (c *Config) FirstLayerBedTemp() []string {
	return c.AsStringArray("first_layer_bed_temperature")
}

func (c *Config) FirstLayerTemp() []string {
	return c.AsStringArray("first_layer_temperature")
}

func (c *Config) RetractLength() []float64 {
	return c.AsFloatArray("retract_length")
}

func (c *Config) RetractSpeed() []float64 {
	return c.AsFloatArray("retract_speed")
}

func (c *Config) SpliceOffset() float64 {
	return c.AsFloat("P2PP_SPLICEOFFSET")
}

func (c *Config) ExtraEndFilament() float64 {
	return c.AsFloat("P2PP_EXTRAENDFILAMENT")
}

func (c *Config) LinearPing() float64 {
	// TODO: need to support configs that don't define linearping
	return c.AsFloat("P2PP_LINEARPING")
}

func (c *Config) PrinterProfileID() string {
	return c.rawvals["P2PP_PRINTERPROFILE"]
}

// how much space between squares to leave
// TODO: make configurable?
func (c *Config) Padding() int {
	return 5
}

// how much space around the edge of the bed area to leave
// TODO: make configurable?
func (c *Config) Margin() int {
	return 10
}

// TODO: this will only support vanilla rectangular beds for now.  Investigate circular and custom.
// ; bed_shape = 0x0,250x0,250x210,0x210
func (c *Config) BedDimensions() image.Rectangle {
	corners := strings.Split(c.rawvals["bed_shape"], ",")
	min := image.Point{math.MaxInt32, math.MaxInt32}
	max := image.Point{0, 0}
	for _, c := range corners {
		v := strings.Split(c, "x")

		x, err := strconv.ParseInt(v[0], 10, 32)
		if err != nil {
			panic(err) // todo: handle gracefully
		}
		y, err := strconv.ParseInt(v[1], 10, 32)
		if err != nil {
			panic(err) // todo: handle gracefully
		}

		if int(x) < min.X {
			min.X = int(x)
		}
		if int(x) > max.X {
			max.X = int(x)
		}
		if int(y) < min.Y {
			min.Y = int(y)
		}
		if int(y) > max.Y {
			max.Y = int(y)
		}
	}
	return image.Rectangle{min, max}
}
