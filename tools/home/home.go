// tools/home/home.go
package home // <-- 包名已修改

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"yanshu-toolkit/core" // <-- 新增导入
)

func init() {
	core.Register(&homeTool{}) // <-- 使用包名调用
}

type homeTool struct{}

func (t *homeTool) Title() string       { return "首页" }
func (t *homeTool) Icon() fyne.Resource { return theme.HomeIcon() }
func (t *homeTool) Category() string    { return "通用" }
func (t *homeTool) View(win fyne.Window) fyne.CanvasObject {
	return container.NewCenter(
		container.NewVBox(
			widget.NewLabelWithStyle("欢迎使用 雁陎工具集", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewIcon(theme.HomeIcon()),
			widget.NewLabel("请从左侧侧边栏选择一个工具开始。"),
		),
	)
}
