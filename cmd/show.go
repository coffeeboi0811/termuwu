package cmd

import (
	"fmt"
	"image"
	_ "image/jpeg" // Support JPEG format
	_ "image/png"  // Support PNG format
	"os"

	"github.com/spf13/cobra"
)

// Command-line flags for 'show'.
var (
	useFullBlocks bool
	useBraille    bool
	noColor       bool // this flag controls dithering.
	renderWidth   int
	renderHeight  int
)

var showCmd = &cobra.Command{
	Use:   "show [image_path]",
	Short: "Render an image in the terminal",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		imagePath := args[0]
		fmt.Printf("📸 Loading image: %s\n", imagePath)

		// Check if one dimension is set but not the other
		if (renderWidth > 0 && renderHeight == 0) || (renderHeight > 0 && renderWidth == 0) {
			fmt.Println("❌ If specifying custom dimensions, both --width (-W) and --height (-H) must be provided.")
			return
		}

		file, err := os.Open(imagePath)
		if err != nil {
			fmt.Printf("❌ Couldn't open image: %v\n", err)
			return
		}
		defer file.Close()

		img, format, err := image.Decode(file)
		if err != nil {
			fmt.Printf("❌ Couldn't decode image (is it a valid PNG/JPEG?): %v\n", err)
			return
		}

		fmt.Printf("✅ Image loaded! Format: %s, Size: %dx%d\n",
			format, img.Bounds().Dx(), img.Bounds().Dy())

		// Figure out the render mode from flags. Default to half-blocks.
		mode := HalfBlockMode
		if useFullBlocks {
			mode = BlockMode
		} else if useBraille {
			mode = BrailleMode
		}

		renderer := NewImageRenderer(mode)
		// 'noColor' flag actually disables dithering.
		renderer.UseDither = !noColor

		// Override dimensions if flags are set
		if renderWidth > 0 {
			renderer.MaxWidth = renderWidth
		}
		if renderHeight > 0 {
			renderer.MaxHeight = renderHeight
		}

		output := renderer.RenderImage(img)
		fmt.Print(output)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)

	showCmd.Flags().BoolVarP(&useFullBlocks, "full", "f", false, "Use full character blocks (less detail).")
	showCmd.Flags().BoolVarP(&useBraille, "braille", "b", false, "Use Braille patterns (experimental, more detail).")
	showCmd.Flags().BoolVarP(&noColor, "no-dither", "n", false, "Disable dithering (can reduce color noise but might cause banding).")
	showCmd.Flags().IntVarP(&renderWidth, "width", "W", 0, "Set the width of the rendered image in characters (0 for auto).")
	showCmd.Flags().IntVarP(&renderHeight, "height", "H", 0, "Set the height of the rendered image in lines (0 for auto).")
}
