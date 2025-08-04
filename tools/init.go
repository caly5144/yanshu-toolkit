/*
 * @Author: Dong Xing
 * @Date: 2025-08-02 19:32:54
 * @LastEditors: Dong Xing
 * @Description: file content
 */
// tools/init.go
package tools

// 这个文件存在的唯一目的，就是利用空白导入来触发
// 各个具体工具子包内的 init() 函数，从而实现自动注册。
// 导入路径现在和物理文件结构完全对应，消除了所有歧义。

import (
	_ "yanshu-toolkit/tools/example"
	_ "yanshu-toolkit/tools/home"
	_ "yanshu-toolkit/tools/image_browser"
	_ "yanshu-toolkit/tools/json_formatter"
	_ "yanshu-toolkit/tools/locallauncher"
	_ "yanshu-toolkit/tools/renamer_tool"
	_ "yanshu-toolkit/tools/text_formatter"
	_ "yanshu-toolkit/tools/timestamp_converter"
)
