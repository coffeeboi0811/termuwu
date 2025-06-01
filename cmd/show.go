package cmd

import (
	"fmt"
	"image"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"os"

	"github.com/spf13/cobra"
)

// Flags for the show command
var (
	useFullBlocks bool // -full: render with full blocks
	useBraille    bool // -braille: render with braille patterns
	noColor       bool // -no-dither: disable dithering (was noColor, but dithering is the color aspect here)
)

var showCmd = &cobra.Command{
	Use:   "show [image_path]",
	Short: "Render an image in the terminal",
	Args:  cobra.ExactArgs(1), // Expect exactly one argument: the image path
	Run: func(cmd *cobra.Command, args []string) {
		imagePath := args[0]
		// Basic user feedback, could be enhanced with a verbose flag later.
		fmt.Printf("📸 Loading image: %s\n", imagePath)

		file, err := os.Open(imagePath)
		if err != nil {
			fmt.Printf("❌ Failed to open image: %v\n", err)
			return
		}
		defer file.Close()

		img, format, err := image.Decode(file)
		if err != nil {
			// This can happen if the file isn't a supported image format or is corrupted.
			fmt.Printf("❌ Failed to decode image: %v\n", err)
			return
		}

		fmt.Printf("✅ Image loaded successfully! Format: %s, Size: %dx%d\n",
			format, img.Bounds().Dx(), img.Bounds().Dy())

		// Determine the rendering mode based on command-line flags.
		// Default to HalfBlockMode for a good balance of quality and performance.
		mode := HalfBlockMode
		if useFullBlocks {
			mode = BlockMode
		} else if useBraille {
			mode = BrailleMode
		}

		renderer := NewImageRenderer(mode)
		// Allow disabling dithering, which can sometimes be useful for specific images or terminals.
		renderer.UseDither = !noColor // Renamed flag from noColor to noDither for clarity

		output := renderer.RenderImage(img)
		fmt.Print(output)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)

	// Define flags for the show command.
	showCmd.Flags().BoolVar(&useFullBlocks, "full", false, "Render using full character blocks (lower vertical resolution).")
	showCmd.Flags().BoolVar(&useBraille, "braille", false, "Render using Unicode Braille patterns (experimental, high detail).")
	// Changed flag name from "no-color" to "no-dither" as dithering is what's being controlled.
	showCmd.Flags().BoolVar(&noColor, "no-dither", false, "Disable dithering to reduce color noise (can lead to banding).")
}
