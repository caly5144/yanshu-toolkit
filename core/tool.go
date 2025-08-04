// core/tool.go
package core
import "fyne.io/fyne/v2"

// Tool 定义了每个工具模块必须提供的功能
type Tool interface {
    Title() string
    Icon() fyne.Resource
    View(win fyne.Window) fyne.CanvasObject
    Category() string
}

// ... 其余部分不变 ...
var toolRegistry []Tool
func Register(t Tool) {
    toolRegistry = append(toolRegistry, t)
}
func GetTools() []Tool {
    return toolRegistry
}