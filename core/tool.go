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

// 【关键修改】: ToolInfo 不再存储 Icon
type ToolInfo struct {
	Title    string
	Category string
	Factory  ToolFactory
}

var toolInfoRegistry []ToolInfo

func RegisterFactory(factory ToolFactory) {
	tempInstance := factory()
	// 【关键修改】: 在注册时，不再调用 Icon() 方法
	info := ToolInfo{
		Title:    tempInstance.Title(),
		Category: tempInstance.Category(),
		Factory:  factory,
	}
	toolInfoRegistry = append(toolInfoRegistry, info)
}

func GetToolInfos() []ToolInfo {
	return toolInfoRegistry
}
