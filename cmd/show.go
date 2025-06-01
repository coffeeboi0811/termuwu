package cmd

import (
	"fmt"
	"image"
	_ "image/gif"  // Support GIF format
	_ "image/jpeg" // Support JPEG format
	_ "image/png"  // Support PNG format
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	_ "golang.org/x/image/webp" // Support WebP format
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
	Use:   "show [image_path_or_url]",
	Short: "Render an image from a local path or URL in the terminal",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		imagePathOrURL := args[0]

		// Check if one dimension is set but not the other
		if (renderWidth > 0 && renderHeight == 0) || (renderHeight > 0 && renderWidth == 0) {
			fmt.Println("❌ If specifying custom dimensions, both --width (-W) and --height (-H) must be provided.")
			return
		}

		var reader io.ReadCloser

		if strings.HasPrefix(imagePathOrURL, "http://") || strings.HasPrefix(imagePathOrURL, "https://") {
			fmt.Printf("📸 Downloading image from URL: %s\n", imagePathOrURL)
			resp, httpErr := http.Get(imagePathOrURL)
			if httpErr != nil {
				fmt.Printf("❌ Couldn't download image: %v\n", httpErr)
				return
			}
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Couldn't download image: received status code %d\n", resp.StatusCode)
				resp.Body.Close()
				return
			}
			reader = resp.Body
		} else {
			fmt.Printf("📸 Loading image from path: %s\n", imagePathOrURL)
			file, fileErr := os.Open(imagePathOrURL)
			if fileErr != nil {
				fmt.Printf("❌ Couldn't open image: %v\n", fileErr)
				return
			}
			reader = file
		}
		defer reader.Close()

		img, format, decodeErr := image.Decode(reader)
		if decodeErr != nil {
			fmt.Printf("❌ Couldn't decode image: %v\n", decodeErr)
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
		// The check above ensures that if one is > 0, both are.
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
