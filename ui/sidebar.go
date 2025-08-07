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
	// 1. 获取所有已注册工具的静态信息
	allToolInfos := core.GetToolInfos()

	// 2. 将工具信息按标题存入 map，以便在布局配置中快速查找
	infoMap := make(map[string]core.ToolInfo)
	for _, info := range allToolInfos {
		infoMap[info.Title] = info
	}

	var accordionItems []*widget.AccordionItem
	for _, categoryLayout := range SidebarLayout {
		buttonList := container.NewVBox()

		for _, toolTitle := range categoryLayout.ToolTitles {
			info, ok := infoMap[toolTitle]
			if !ok {
				continue
			}

			// 【关键修改】: 在按钮的回调函数中，调用工厂来创建新实例
			btn := widget.NewButtonWithIcon(info.Title, info.Icon, func() {
				// 每次点击，都通过工厂创建一个全新的工具实例！
				newToolInstance := info.Factory()
				openTab(content, newToolInstance, win)
			})

			btn.Alignment = widget.ButtonAlignLeading
			indentation := canvas.NewRectangle(color.Transparent)
			indentation.SetMinSize(fyne.NewSize(16, 0))
			indentedButtonContainer := container.NewBorder(nil, nil, indentation, nil, btn)
			buttonList.Add(indentedButtonContainer)
		}

		if len(buttonList.Objects) > 0 {
			item := widget.NewAccordionItem(categoryLayout.Name, buttonList)
			accordionItems = append(accordionItems, item)
		}
	}

	accordion := widget.NewAccordion(accordionItems...)
	if len(accordion.Items) > 0 {
		accordion.Open(0)
	}

	return container.NewScroll(accordion)
}
