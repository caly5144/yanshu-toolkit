// core/tool.go
package core

import "fyne.io/fyne/v2"

// Tool 接口保持不变
type Tool interface {
	Title() string
	Icon() fyne.Resource
	View(win fyne.Window) fyne.CanvasObject
	Category() string
	Destroy()
}

// ToolFactory 函数类型保持不变
type ToolFactory func() Tool

// 【关键修改】: 将 toolInfo 和其字段名改为大写，以便导出
type ToolInfo struct {
	Title    string
	Category string
	Icon     fyne.Resource
	Factory  ToolFactory
}

// 私有变量，存储注册信息
var toolInfoRegistry []ToolInfo // <-- 类型也改为大写的 ToolInfo

// RegisterFactory 函数保持不变，但内部使用的类型需要更新
func RegisterFactory(factory ToolFactory) {
	tempInstance := factory()
	// 【关键修改】: 使用大写的 ToolInfo 和字段名
	info := ToolInfo{
		Title:    tempInstance.Title(),
		Category: tempInstance.Category(),
		Icon:     tempInstance.Icon(),
		Factory:  factory,
	}
	toolInfoRegistry = append(toolInfoRegistry, info)
}

// GetToolInfos 返回所有已注册工具的静态信息列表
// 【关键修改】: 返回类型改为 []ToolInfo
func GetToolInfos() []ToolInfo {
	return toolInfoRegistry
}
