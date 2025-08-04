// ui/sidebar.go
package ui

import (
	"image/color"

	"yanshu-toolkit/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func createSidebar(content *container.DocTabs, win fyne.Window) fyne.CanvasObject {
	// 1. 获取所有已注册的工具，并放入一个 map 中以便快速查找
	allTools := core.GetTools()
	toolMap := make(map[string]core.Tool)
	for _, t := range allTools {
		toolMap[t.Title()] = t
	}

	var accordionItems []*widget.AccordionItem
	for _, categoryLayout := range SidebarLayout {
		buttonList := container.NewVBox()

		// 遍历指定顺序的工具标题
		for _, toolTitle := range categoryLayout.ToolTitles {
			tool, ok := toolMap[toolTitle]
			if !ok {
				continue // 如果工具未注册，安全跳过
			}

			// --- 创建带缩进的按钮容器 (逻辑不变) ---
			// 注意：这里的 btn 回调函数现在需要捕获 tool 变量
			btn := widget.NewButtonWithIcon(tool.Title(), tool.Icon(), func() {
				// 在回调函数中，调用 openTab 并传入 win
				openTab(content, tool, win)
			})
			btn.Alignment = widget.ButtonAlignLeading

			indentation := canvas.NewRectangle(color.Transparent)
			indentation.SetMinSize(fyne.NewSize(16, 0))

			indentedButtonContainer := container.NewBorder(nil, nil, indentation, nil, btn)
			buttonList.Add(indentedButtonContainer)
		}

		// 只有当分组内至少有一个成功渲染的工具时，才创建该分组
		if len(buttonList.Objects) > 0 {
			item := widget.NewAccordionItem(categoryLayout.Name, buttonList)
			accordionItems = append(accordionItems, item)
		}
	}

	accordion := widget.NewAccordion(accordionItems...)

	// 默认展开第一个在布局中定义的分组
	if len(accordion.Items) > 0 {
		accordion.Open(0)
	}

	scrollableAccordion := container.NewScroll(accordion)
	return scrollableAccordion
}
