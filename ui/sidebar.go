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
	allToolInfos := core.GetToolInfos()
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

			// --- 【关键修改】: 在这里延迟获取图标 ---
			// 1. 创建一个临时的、一次性的工具实例，目的仅仅是为了调用它的 Icon() 方法。
			//    因为此时 App 已经运行，所以这是安全的。
			tempToolForIcon := info.Factory()
			toolIcon := tempToolForIcon.Icon()

			// 2. 使用获取到的图标创建按钮。
			//    注意：按钮的回调函数仍然是创建一个全新的实例，绝不能使用 tempToolForIcon！
			btn := widget.NewButtonWithIcon(info.Title, toolIcon, func() {
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
