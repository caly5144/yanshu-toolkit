// theme/theme.go
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"yanshu-toolkit/assets" // 导入我们新的 assets 包
)

var (
	// --- 字体资源定义 ---
	// 直接使用从 assets 包导入的 byte slice 来创建资源
	resourceSourceHanRegular = &fyne.StaticResource{StaticName: "SourceHanSerif-Regular.otf", StaticContent: assets.FontSourceHanRegular}
	// resourceSourceHanBold    = &fyne.StaticResource{StaticName: "SourceHanSerif-Bold.otf", StaticContent: assets.FontSourceHanBold}

	// --- 图标资源定义 ---
	// 同样，直接使用 assets 包的数据
	sunResource  = &fyne.StaticResource{StaticName: "sun.svg", StaticContent: assets.IconSunSVG}
	moonResource = &fyne.StaticResource{StaticName: "moon.svg", StaticContent: assets.IconMoonSVG}

	// 使用 theme.NewThemedResource 将其包装成主题资源
	SunIcon  fyne.Resource = theme.NewThemedResource(sunResource)
	MoonIcon fyne.Resource = theme.NewThemedResource(moonResource)
)

type fontAwareTheme struct {
	fyne.Theme
}

func (t *fontAwareTheme) Font(style fyne.TextStyle) fyne.Resource {
	// if style.Bold {
	//     return resourceSourceHanBold
	// }
	return resourceSourceHanRegular
}

// --- DarkTheme (黑夜模式) (保持不变) ---
type darkTheme struct {
	fontAwareTheme
}

func (t *darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 0, G: 120, B: 215, A: 255}
	}
	if name == theme.ColorNameBackground {
		return color.NRGBA{R: 32, G: 32, B: 32, A: 255}
	}
	return theme.DarkTheme().Color(name, variant)
}

func NewDarkTheme() fyne.Theme {
	return &darkTheme{fontAwareTheme{Theme: theme.DarkTheme()}}
}

// --- LightTheme (白天模式) (保持不变) ---
type lightTheme struct {
	fontAwareTheme
}

func (t *lightTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 0, G: 120, B: 215, A: 255}
	}
	return theme.LightTheme().Color(name, variant)
}

func NewLightTheme() fyne.Theme {
	return &lightTheme{fontAwareTheme{Theme: theme.LightTheme()}}
}
