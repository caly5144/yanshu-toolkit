// tools/example/example.go
package example // <-- 包名已修改

import (
    "fmt"
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "yanshu-toolkit/core" // <-- 新增导入
)

func init() {
    core.Register(&exampleTool{}) // <-- 使用包名调用
}

type exampleTool struct{}

func (t *exampleTool) Title() string       { return "简单示例" }
func (t *exampleTool) Icon() fyne.Resource { return theme.InfoIcon() }
func (t *exampleTool) Category() string    { return "通用" }
func (t *exampleTool) View(win fyne.Window) fyne.CanvasObject {
    entry := widget.NewEntry()
    entry.SetPlaceHolder("在这里输入一些内容...")
    label := widget.NewLabel("状态将会被保留")
    btn := widget.NewButton("更新标签", func() {
        label.SetText(fmt.Sprintf("你输入了: %s", entry.Text))
    })

    return container.NewVBox(
        widget.NewLabelWithStyle("这是一个简单的示例", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        widget.NewSeparator(),
        entry,
        btn,
        layout.NewSpacer(),
        label,
    )
}