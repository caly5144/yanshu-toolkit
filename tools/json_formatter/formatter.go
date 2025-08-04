// tools/json_formatter/formatter.go
package json_formatter

import (
    "bytes"
    "encoding/json"
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"
    "yanshu-toolkit/core"
)

func init() {
    core.Register(&jsonFormatterTool{})
}

type jsonFormatterTool struct{}

func (t *jsonFormatterTool) Title() string       { return "JSON格式化" }
func (t *jsonFormatterTool) Icon() fyne.Resource { return theme.ListIcon() }
func (t *jsonFormatterTool) Category() string    { return "常用工具" } // 新的分组名

func (t *jsonFormatterTool) View(win fyne.Window) fyne.CanvasObject {
    input := widget.NewMultiLineEntry()
    input.SetPlaceHolder("在此粘贴 JSON 文本...")
    input.Wrapping = fyne.TextWrapOff

    formatBtn := widget.NewButton("格式化", func() {
        var prettyJSON bytes.Buffer
        err := json.Indent(&prettyJSON, []byte(input.Text), "", "  ")
        if err != nil {
            // 如果格式化失败，显示错误弹窗
            // 这里我们找到顶层窗口来显示对话框
            win := fyne.CurrentApp().Driver().AllWindows()[0]
            dialog.ShowError(err, win)
            return
        }
        input.SetText(prettyJSON.String())
    })

    return container.NewBorder(nil, formatBtn, nil, nil, container.NewScroll(input))
}