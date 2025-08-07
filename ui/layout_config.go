/*
 * @Author: Dong Xing
 * @Date: 2025-08-04 22:40:35
 * @LastEditors: Dong Xing
 * @Description: file content
 */
// ui/layout_config.go
package ui

// CategoryLayout 定义了侧边栏中一个分组的结构
type CategoryLayout struct {
	Name       string   // 分组的显示名称，例如 "通用"
	ToolTitles []string // 该分组下所有工具的标题，此处的顺序就是最终的显示顺序
}

// SidebarLayout 是整个侧边栏的布局定义。
// 您可以通过修改这个变量来轻松地调整分组顺序、工具顺序，或者在不同分组间移动工具。
var SidebarLayout = []CategoryLayout{
	{
		Name: "通用",
		ToolTitles: []string{
			"首页",   // 第一个
			"简单示例", // 第二个
		},
	},
	{
		Name: "常用工具",
		ToolTitles: []string{
			"JSON格式化",
			"本地启动器",
		},
	},
	{
		Name: "文本工具",
		ToolTitles: []string{
			"排版助手",
		},
	},
	{
		Name: "文件管理",
		ToolTitles: []string{
			"批量重命名",
		},
	},
	{
		Name: "媒体工具",
		ToolTitles: []string{
			"图片浏览器",
		},
	},
}
