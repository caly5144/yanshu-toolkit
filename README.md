# yanshu-toolkit 自用工具集

这是一个使用 Go和[Fyne](https://fyne.io/) 构建的自用桌面工具集，Gemini贡献了绝大部分代码。

---

## ✨ 功能亮点

*   **暂无**: 自用而已，尚未开发完成。

## 🛠️ 技术栈

*   **核心框架**: [Fyne v2](https://docs.fyne.io/)
*   **编程语言**: [Go](https://golang.org/)

## 📂 项目结构

项目采用清晰的分层和模块化结构，易于理解和维护。

```
yanshu-toolkit/
├── assets/                 # 存放所有静态资源
│   ├── font/               # 字体文件 (未经过子集化处理)
│   ├── icon/               # SVG 图标文件
│   └── assets.go           # 统一加载并暴露资源的 Go 文件
├── core/                   # 核心定义包
│   └── tool.go             # 定义 Tool 接口和工具注册表
├── theme/                  # 主题和样式包
│   └── theme.go            # 定义白天/黑夜主题、加载字体和图标资源
├── tools/                  # 所有工具模块
│   ├── home/               # “首页”工具的实现
│   ├── example/            # “简单示例”工具的实现
│   ├── json_formatter/     # ...更多工具
│   └── init.go             # 通过空白导入，自动注册所有工具
├── ui/                     # UI 布局和组件包
│   ├── sidebar.go          # 负责构建侧边栏
│   ├── layout_config.go    # 侧边栏布局和工具顺序的配置文件 <--- 调整布局的核心
│   └── layout.go           # 构建主窗口的整体布局
├── go.mod                  # Go 模块文件
└── main.go                 # 程序主入口
```

## 🚀 如何运行

### 1. 准备环境

*   确保您已经安装了 [Go (1.18 或更高版本)](https://golang.org/doc/install)。
*   确保您的系统满足 [Fyne (2.6.2) 的环境要求](https://docs.fyne.io/started/)（例如，在 Windows 上需要安装 MSYS2）。
*   (可选，仅用于字体子集化) 安装 [Python](https://www.python.org/) 和 `fonttools`：
    ```bash
    pip install fonttools
    ```

### 2. 克隆并运行

```bash
# 1. 克隆仓库
git clone https://github.com/your-username/yanshu-toolkit.git
cd yanshu-toolkit

# 2. 下载依赖
go mod tidy

# 3. 运行程序
go run .
```

### 3. 如何打包

使用 `fyne` 命令行工具可以轻松地将应用打包成可执行文件。

```bash
# 1. 安装 Fyne 命令行工具
go install fyne.io/fyne/v2/cmd/fyne@latest

# 2. 启动应用
go run .

# 2. 打包应用（会自动包含图标和元数据）
#    -ldflags="-H windowsgui" 用于在 Windows 上隐藏命令行窗口
go build -ldflags "-s -w -H=windowsgui"
```

## 🧩 如何扩展：添加一个新工具

得益于模块化的设计，添加一个新工具非常简单：

1.  **创建工具包**:
    在 `tools/` 目录下创建一个新的文件夹，例如 `my_new_tool`。

2.  **实现 `Tool` 接口**:
    在 `tools/my_new_tool/` 目录下创建一个 `tool.go` 文件。

    ```go
    // tools/my_new_tool/tool.go
    package my_new_tool

    import (
        "fyne.io/fyne/v2"
        "fyne.io/fyne/v2/widget"
        "yanshu-toolkit/core"
    )

    func init() {
        // 在 init 函数中自动注册自己
        core.Register(&myTool{})
    }
    
    type myTool struct{}

    func (t *myTool) Title() string       { return "我的新工具" }
    func (t *myTool) Icon() fyne.Resource { /* 返回一个图标 */ }
    func (t *myTool) Category() string    { return "其他工具" } // 可自定义分类
    func (t *myTool) View(win fyne.Window) fyne.CanvasObject {
        return widget.NewLabel("这是我的新工具界面！")
    }
    ```

3.  **进行空白导入**:
    打开 `tools/init.go` 文件，在文件底部添加一行对新工具包的空白导入，以触发其 `init()` 函数。
    ```go
    // tools/init.go
    import (
        // ... 其他导入
        _ "yanshu-toolkit/tools/my_new_tool" // <-- 新增此行
    )
    ```

4.  **配置布局**:
    打开 `ui/layout_config.go` 文件，将新工具的标题 `"我的新工具"` 添加到您希望它出现的分类和位置。
    ```go
    // ui/layout_config.go
    var SidebarLayout = []CategoryLayout{
        // ... 其他分组
        {
            Name: "其他工具",
            ToolTitles: []string{
                "我的新工具", // <-- 将标题放在这里
            },
        },
    }
    ```

**完成！** 重新运行程序，您的新工具就已经无缝集成到 UI 中了。

## 🐛 已知问题

1. 排版助手渲染异常，会出现字体重叠、首行缩进无效果的情况，皆会出现在长段落中，不过这**仅仅是一个视觉渲染问题**，后台数据和复制到剪贴板的内容是完全正确的。

## 致谢

*   感谢 [Fyne](https://fyne.io/) 开发的GUI 框架。
*   感谢 [Adobe Fonts](https://github.com/adobe-fonts/source-han-serif) 的开源字体“思源宋体”。

---