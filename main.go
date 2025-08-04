// main.go
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	// 导入本地包
	appTheme "yanshu-toolkit/theme"
	"yanshu-toolkit/ui"

	// 只需导入 tools 包，它会通过自己的 init.go 文件自动注册所有工具
	_ "yanshu-toolkit/tools"
)

func main() {
	myApp := app.NewWithID("com.yanshu.toolkit")
	myApp.Settings().SetTheme(appTheme.NewLightTheme()) // 初始设置为亮色主题

	myWindow := myApp.NewWindow("雁陎工具集")
	myWindow.Resize(fyne.NewSize(1024, 768))

	// 【修改点】: 将 myWindow 传递给布局函数
	mainLayout := ui.CreateMainWindowLayout(myApp, myWindow)
	myWindow.SetContent(mainLayout)

	myWindow.ShowAndRun()
}
