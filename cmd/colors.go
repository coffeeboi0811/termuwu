package cmd

type Color struct {
	R, G, B uint8
}

// rGBToANSI256 tries to find the best ANSI 256 color for a given RGB.
// input r, g, b are 0-65535
func RGBToANSI256(r, g, b uint32) int {
	r8 := clamp8(r >> 8)
	g8 := clamp8(g >> 8)
	b8 := clamp8(b >> 8)

	if r8 == 0 && g8 == 0 && b8 == 0 {
		return 16 // ansi black
	}

	// nudge very dark colors up a bit
	if r8 < 15 && g8 < 15 && b8 < 15 {
		r8 = clamp8(uint32(r8) + 10)
		g8 = clamp8(uint32(g8) + 10)
		b8 = clamp8(uint32(b8) + 10)
	}

	if isGrayscale(r8, g8, b8) {
		return mapToGrayscale(r8, g8, b8)
	}

	rLevel := quantizeToSix(r8)
	gLevel := quantizeToSix(g8)
	bLevel := quantizeToSix(b8)

	cubeColor := 16 + (36 * rLevel) + (6 * gLevel) + bLevel

	// for near-grays, the dedicated grayscale ramp can sometimes be a better fit
	if isNearGrayscale(r8, g8, b8) {
		grayColor := mapToGrayscale(r8, g8, b8)
		if colorDistance(r8, g8, b8, grayColor) < colorDistance(r8, g8, b8, cubeColor) {
			return grayColor
		}
	}

	return cubeColor
}

func clamp8(val uint32) uint8 {
	if val > 255 {
		return 255
	}
	return uint8(val)
}

// quantizeToSix maps an 8-bit color to one of 6 ANSI cube levels
func quantizeToSix(val uint8) int {
	if val < 48 { // thresholds for 6 levels (0..5)
		return 0
	}
	if val < 95 {
		return 1
	}
	if val < 142 {
		return 2
	}
	if val < 189 {
		return 3
	}
	if val < 236 {
		return 4
	}
	return 5
}

func isGrayscale(r, g, b uint8) bool {
	max := maxUint8(r, g, b)
	min := minUint8(r, g, b)
	return max-min <= 10 // tolerance for "gray enough"
}

func isNearGrayscale(r, g, b uint8) bool {
	max := maxUint8(r, g, b)
	min := minUint8(r, g, b)
	return max-min <= 30 // a bit more forgiving for "almost gray"
}

// mapToGrayscale finds the closest match in ANSI's 24-step grayscale ramp
func mapToGrayscale(r, g, b uint8) int {
	gray := uint8(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) // standard luminance

	if gray < 10 {
		return 232 // darkest non-black gray
	}
	if gray > 245 {
		return 255 // lightest gray
	}
	return 232 + int(float64(gray-8)*(23.0/240.0))
}

// colorDistance estimates perceptual difference between colors
func colorDistance(r1, g1, b1 uint8, colorIndex int) float64 {
	r2, g2, b2 := ansiToRGB(colorIndex)

	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)

	// weighted to roughly match human perception
	return 2*dr*dr + 4*dg*dg + 3*db*db
}

// ansiToRGB converts an ANSI index back to an approximate RGB
func ansiToRGB(index int) (uint8, uint8, uint8) {
	if index >= 232 && index <= 255 { // grays
		grayVal := float64(index-232)*(240.0/23.0) + 8.0
		if grayVal < 0 {
			grayVal = 0
		}
		if grayVal > 255 {
			grayVal = 255
		}
		return uint8(grayVal), uint8(grayVal), uint8(grayVal)
	}

	if index >= 16 && index <= 231 { // 6x6x6 color cube
		index -= 16
		r := (index / 36) % 6
		g := (index / 6) % 6
		b := index % 6
		valMap := []uint8{0, 47, 95, 142, 189, 236} // rgb values for cube levels
		return valMap[r], valMap[g], valMap[b]
	}

	// standard 0-15 colors vary by terminal
	if index == 0 || index == 16 {
		return 0, 0, 0 // black
	}
	if index == 7 || index == 231 {
		return 255, 255, 255 // white
	}
	return 128, 128, 128 // default to mid-gray
}

func maxUint8(a, b, c uint8) uint8 {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

func minUint8(a, b, c uint8) uint8 {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}
