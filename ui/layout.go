/*
 * @Author: Dong Xing
 * @Date: 2025-08-02 18:22:25
 * @LastEditors: Dong Xing
 * @Description: file content
 */
// ui/layout.go
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"yanshu-toolkit/core"
	appTheme "yanshu-toolkit/theme"
)

func openTab(tabs *container.DocTabs, tool core.Tool, win fyne.Window) {
	for _, item := range tabs.Items {
		if item.Text == tool.Title() {
			tabs.Select(item)
			return
		}
	}
	tab := container.NewTabItemWithIcon(tool.Title(), tool.Icon(), tool.View(win))
	tabs.Append(tab)
	tabs.Select(tab)
}

func CreateMainWindowLayout(app fyne.App, win fyne.Window) fyne.CanvasObject {
	mainContent := container.NewDocTabs()
	mainContent.SetTabLocation(container.TabLocationTop)

	sidebar := createSidebar(mainContent, win)

	split := container.NewHSplit(sidebar, mainContent)
	split.Offset = 0.2

	isDarkMode := false // 假设初始是亮色
	themeToggleBtn := widget.NewButtonWithIcon("", appTheme.MoonIcon, nil)

	themeToggleBtn.OnTapped = func() {
		if isDarkMode {
			app.Settings().SetTheme(appTheme.NewLightTheme())
			themeToggleBtn.SetIcon(appTheme.MoonIcon)
		} else {
			app.Settings().SetTheme(appTheme.NewDarkTheme())
			themeToggleBtn.SetIcon(appTheme.SunIcon)
		}
		isDarkMode = !isDarkMode
	}

	buttonOverlay := container.NewBorder(
		container.NewHBox(layout.NewSpacer(), themeToggleBtn),
		nil, nil, nil,
	)

	finalLayout := container.NewMax(split, buttonOverlay)

	// 默认打开首页
	for _, t := range core.GetTools() {
		if t.Title() == "首页" {
			openTab(mainContent, t, win)
			break
		}
	}

	return finalLayout
}
