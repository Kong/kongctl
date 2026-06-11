package art

import (
	"embed"
	"fmt"
	"io"
	"slices"
)

//go:embed assets/kong-*-ascii.txt assets/kong-*-braille.txt
var assets embed.FS

type KongBannerType string

const (
	KongBannerASCII   KongBannerType = "ascii"
	KongBannerBraille KongBannerType = "braille"
)

var supportedKongBannerWidths = []int{48, 88, 104, 120}
var supportedKongBannerTypes = []KongBannerType{KongBannerASCII, KongBannerBraille}

func (t KongBannerType) String() string {
	return string(t)
}

func SupportedKongBannerWidths() []int {
	return slices.Clone(supportedKongBannerWidths)
}

func SupportedKongBannerTypes() []KongBannerType {
	return slices.Clone(supportedKongBannerTypes)
}

func RenderLoginBanner(w io.Writer) error {
	return RenderKongBanner(w, 48)
}

func RenderKongBanner(w io.Writer, width int) error {
	return RenderKongBannerType(w, width, KongBannerBraille)
}

func RenderKongBannerType(w io.Writer, width int, bannerType KongBannerType) error {
	if w == nil {
		return nil
	}
	if !slices.Contains(supportedKongBannerWidths, width) {
		return fmt.Errorf("unsupported kong banner width %d; supported widths: %v", width, supportedKongBannerWidths)
	}
	if !slices.Contains(supportedKongBannerTypes, bannerType) {
		return fmt.Errorf("unsupported kong banner type %q; supported types: %v", bannerType, supportedKongBannerTypes)
	}

	return renderAsset(w, fmt.Sprintf("assets/kong-%d-%s.txt", width, bannerType))
}

func renderAsset(w io.Writer, name string) error {
	content, err := assets.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read banner asset %q: %w", name, err)
	}
	if _, err := w.Write(content); err != nil {
		return fmt.Errorf("write banner asset %q: %w", name, err)
	}
	if len(content) == 0 || content[len(content)-1] != '\n' {
		if _, err := fmt.Fprintln(w); err != nil {
			return fmt.Errorf("terminate banner asset %q: %w", name, err)
		}
	}
	return nil
}
