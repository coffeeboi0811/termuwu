package cmd

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	_ "golang.org/x/image/webp"
)

var (
	useFullBlocks bool
	useBraille    bool
	noDither      bool
	renderWidth   int
	renderHeight  int
)

func loadImage(pathOrURL string) (image.Image, string, error) {
	var reader io.ReadCloser
	cyan := color.New(color.FgCyan).SprintFunc()
	urlColor := color.New(color.FgBlue, color.Underline).SprintFunc()

	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		fmt.Printf("📸 %s %s\n", cyan("Downloading image from URL:"), urlColor(pathOrURL))
		req, _ := http.NewRequest("GET", pathOrURL, nil)
		resp, httpErr := http.DefaultClient.Do(req)
		if httpErr != nil {
			return nil, "", fmt.Errorf("couldn't download image: %w", httpErr)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, "", fmt.Errorf("couldn't download image: received status code %d", resp.StatusCode)
		}

		barGreen := color.New(color.FgGreen).SprintFunc()
		barLightBlack := color.New(color.FgHiBlack).SprintFunc()

		bar := progressbar.NewOptions64(
			resp.ContentLength,
			progressbar.OptionSetDescription(cyan("Downloading...")),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetWidth(25),
			progressbar.OptionShowBytes(true),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionSetItsString("bytes"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        barGreen("█"),
				SaucerHead:    barGreen("█"),
				SaucerPadding: barLightBlack("░"),
				BarStart:      "|",
				BarEnd:        "|",
			}),
		)
		reader = io.NopCloser(io.TeeReader(resp.Body, bar))
	} else {
		fmt.Printf("📸 %s %s\n", cyan("Loading image from path:"), pathOrURL)
		file, fileErr := os.Open(pathOrURL)
		if fileErr != nil {
			return nil, "", fmt.Errorf("couldn't open image: %w", fileErr)
		}
		reader = file
	}
	defer reader.Close()

	img, format, decodeErr := image.Decode(reader)
	if decodeErr != nil {
		return nil, "", fmt.Errorf("couldn't decode image: %w", decodeErr)
	}
	return img, format, nil
}

func configureRenderer(useFullBlocksFlag, useBrailleFlag, noDitherFlag bool, widthFlag, heightFlag int) *ImageRenderer {
	mode := HalfBlockMode
	if useFullBlocksFlag {
		mode = BlockMode
	} else if useBrailleFlag {
		mode = BrailleMode
	}

	renderer := NewImageRenderer(mode)
	renderer.UseDither = !noDitherFlag

	if widthFlag > 0 {
		renderer.MaxWidth = widthFlag
	}
	if heightFlag > 0 {
		renderer.MaxHeight = heightFlag
	}
	return renderer
}

var showCmd = &cobra.Command{
	Use:   "show [image_path_or_url]",
	Short: "Render an image from a local path or URL in the terminal",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		imagePathOrURL := args[0]

		errorColor := color.New(color.FgRed, color.Bold).SprintFunc()
		successColor := color.New(color.FgGreen).SprintFunc()
		infoColor := color.New(color.FgYellow).SprintFunc()

		if (renderWidth > 0 && renderHeight == 0) || (renderHeight > 0 && renderWidth == 0) {
			fmt.Println(errorColor("❌ If specifying custom dimensions, both --width (-W) and --height (-H) must be provided."))
			return
		}

		img, format, err := loadImage(imagePathOrURL)
		if err != nil {
			fmt.Printf("%s %v\n", errorColor("❌ Error loading image:"), err)
			return
		}

		if strings.HasPrefix(imagePathOrURL, "http://") || strings.HasPrefix(imagePathOrURL, "https://") {
			fmt.Println()
		}

		fmt.Printf("✅ %s Format: %s, Size: %dx%d\n",
			successColor("Image loaded!"),
			infoColor(format),
			img.Bounds().Dx(),
			img.Bounds().Dy())

		renderer := configureRenderer(useFullBlocks, useBraille, noDither, renderWidth, renderHeight)

		output := renderer.RenderImage(img)
		fmt.Print(output)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)

	showCmd.Flags().BoolVarP(&useFullBlocks, "full", "f", false, "Use full character blocks (less detail).")
	showCmd.Flags().BoolVarP(&useBraille, "braille", "b", false, "Use Braille patterns (experimental, more detail).")
	showCmd.Flags().BoolVarP(&noDither, "no-dither", "n", false, "Disable dithering (can reduce color noise but might cause banding).")
	showCmd.Flags().IntVarP(&renderWidth, "width", "W", 0, "Set the width of the rendered image in characters (0 for auto).")
	showCmd.Flags().IntVarP(&renderHeight, "height", "H", 0, "Set the height of the rendered image in lines (0 for auto).")
}
