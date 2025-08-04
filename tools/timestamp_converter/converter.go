// tools/timestamp_converter/converter.go
package timestamp_converter

import (
    // "fmt"
    "strconv"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"
    "yanshu-toolkit/core"
)

func init() {
    core.Register(&timestampConverterTool{})
}

type timestampConverterTool struct{}

func (t *timestampConverterTool) Title() string       { return "时间戳转换" }
func (t *timestampConverterTool) Icon() fyne.Resource { return theme.HistoryIcon() }
func (t *timestampConverterTool) Category() string    { return "常用工具" } // 新的分组名

func (t *timestampConverterTool) View(win fyne.Window) fyne.CanvasObject {
    entry := widget.NewEntry()
    entry.SetPlaceHolder("输入 Unix 时间戳 (秒)...")

    resultLabel := widget.NewLabelWithStyle("转换结果将显示在这里", fyne.TextAlignCenter, fyne.TextStyle{})

    convertBtn := widget.NewButton("转换", func() {
        ts, err := strconv.ParseInt(entry.Text, 10, 64)
        if err != nil {
            resultLabel.SetText("错误: 无效的数字")
            return
        }
        t := time.Unix(ts, 0)
        resultLabel.SetText(t.Format("2006-01-02 15:04:05"))
    })

    return container.NewVBox(
        widget.NewLabelWithStyle("时间戳转换器", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        widget.NewSeparator(),
        container.NewGridWithColumns(2, entry, convertBtn),
        resultLabel,
    )
}