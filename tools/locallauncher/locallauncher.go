package locallauncher

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"yanshu-toolkit/core" // 请确保这里的路径是您项目中 core 包的正确导入路径

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	configDir      = "./data"
	configFileName = "locallauncher.json"
)

// ToolConfig 定义了单个工具的配置结构
type ToolConfig struct {
	Name     string `json:"name"`
	ExecPath string `json:"exec_path"`
	IconPath string `json:"icon_path"`
}

// localLauncherTool 是我们工具的主结构
type localLauncherTool struct {
	configs   []ToolConfig
	win       fyne.Window
	container *fyne.Container
}

// 注册工具
func init() {
	core.Register(&localLauncherTool{})
}

// --- 实现 core.Tool 接口 ---

func (t *localLauncherTool) Title() string {
	return "本地工具"
}

func (t *localLauncherTool) Icon() fyne.Resource {
	return theme.StorageIcon()
}

func (t *localLauncherTool) Category() string {
	return "常用工具"
}

func (t *localLauncherTool) View(win fyne.Window) fyne.CanvasObject {
	t.win = win
	t.loadConfig()

	t.container = container.NewStack()
	t.refreshUI()

	return t.container
}

// --- 配置管理 ---

func (t *localLauncherTool) configPath() string {
	return filepath.Join(configDir, configFileName)
}

func (t *localLauncherTool) loadConfig() {
	path := t.configPath()
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("配置文件不存在，将创建一个新的。")
			t.configs = []ToolConfig{}
			return
		}
		log.Printf("读取配置文件失败: %v", err)
		t.configs = []ToolConfig{}
		return
	}

	err = json.Unmarshal(data, &t.configs)
	if err != nil {
		log.Printf("解析配置文件失败: %v", err)
		t.configs = []ToolConfig{}
	}
}

func (t *localLauncherTool) saveConfig() {
	path := t.configPath()
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
	}

	data, err := json.MarshalIndent(t.configs, "", "  ")
	if err != nil {
		log.Printf("序列化配置失败: %v", err)
		return
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		log.Printf("写入配置文件失败: %v", err)
	}
}

// --- UI 构建与刷新 ---

func (t *localLauncherTool) refreshUI() {
	if len(t.configs) == 0 {
		t.container.Objects = []fyne.CanvasObject{t.createEmptyView()}
	} else {
		t.container.Objects = []fyne.CanvasObject{t.createGridView()}
	}
	t.container.Refresh()
}

func (t *localLauncherTool) createEmptyView() fyne.CanvasObject {
	return container.NewCenter(
		container.NewVBox(
			widget.NewLabelWithStyle("尚未配置任何本地工具", fyne.TextAlignCenter, fyne.TextStyle{}),
			layout.NewSpacer(),
			container.NewCenter(
				widget.NewButtonWithIcon("添加第一个工具", theme.ContentAddIcon(), func() {
					t.showEditDialog(-1)
				}),
			),
		),
	)
}

// <-- 修改点: 移除了顶部的排序按钮和相关布局
func (t *localLauncherTool) createGridView() fyne.CanvasObject {
	// --- 创建网格内容 ---
	gridContent := make([]fyne.CanvasObject, 0)
	for i := range t.configs {
		index := i
		gridContent = append(gridContent, t.createToolButton(index))
	}

	addBtn := container.NewVBox(
		widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
			t.showEditDialog(-1)
		}),
		widget.NewLabelWithStyle("添加", fyne.TextAlignCenter, fyne.TextStyle{}),
	)
	gridContent = append(gridContent, addBtn)

	// --- 直接返回可滚动的网格容器 ---
	return container.NewScroll(container.NewGridWrap(fyne.NewSize(100, 100), gridContent...))
}

func (t *localLauncherTool) createToolButton(index int) fyne.CanvasObject {
	conf := t.configs[index]
	toolBtn := newRightClickableButton(conf.Name, t.getIconResource(conf.IconPath))

	toolBtn.OnTapped = func() {
		log.Printf("正在启动: %s", conf.ExecPath)
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/C", "start", "", conf.ExecPath)
		} else {
			cmd = exec.Command(conf.ExecPath)
			cmd.Dir = filepath.Dir(conf.ExecPath)
		}
		err := cmd.Start()
		if err != nil {
			log.Printf("启动 %s 失败: %v", conf.Name, err)
			dialog.ShowError(err, t.win)
		}
	}

	toolBtn.OnTappedSecondary = func(pe *fyne.PointEvent) {
		renameItem := fyne.NewMenuItem("重命名", func() {
			t.showRenameDialog(index)
		})
		changeIconItem := fyne.NewMenuItem("修改图标", func() {
			t.showChangeIconDialog(index)
		})

		moveUpItem := fyne.NewMenuItem("上移", func() {
			if index > 0 {
				t.configs[index], t.configs[index-1] = t.configs[index-1], t.configs[index]
				t.saveConfig()
				t.refreshUI()
			}
		})
		moveDownItem := fyne.NewMenuItem("下移", func() {
			if index < len(t.configs)-1 {
				t.configs[index], t.configs[index+1] = t.configs[index+1], t.configs[index]
				t.saveConfig()
				t.refreshUI()
			}
		})

		if index == 0 {
			moveUpItem.Disabled = true
		}
		if index == len(t.configs)-1 {
			moveDownItem.Disabled = true
		}

		deleteItem := fyne.NewMenuItem("删除", func() {
			t.showDeleteDialog(index)
		})

		popup := widget.NewPopUpMenu(fyne.NewMenu("",
			renameItem,
			changeIconItem,
			fyne.NewMenuItemSeparator(),
			moveUpItem,
			moveDownItem,
			fyne.NewMenuItemSeparator(),
			deleteItem,
		), t.win.Canvas())
		popup.ShowAtPosition(pe.AbsolutePosition)
	}

	return container.NewVBox(
		toolBtn,
		widget.NewLabelWithStyle(conf.Name, fyne.TextAlignCenter, fyne.TextStyle{}),
	)
}

// --- 对话框 (无变化) ---
func (t *localLauncherTool) showEditDialog(index int) {
	execPathEntry := widget.NewEntry()
	execPathEntry.SetPlaceHolder("点击右侧按钮选择可执行文件...")
	nameEntry := widget.NewEntry()
	iconPathEntry := widget.NewEntry()
	iconPathEntry.SetPlaceHolder("(可选) 点击右侧按钮选择图标文件...")
	dialogTitle := "添加新工具"
	if index >= 0 {
		dialogTitle = "编辑工具"
		conf := t.configs[index]
		execPathEntry.SetText(conf.ExecPath)
		nameEntry.SetText(conf.Name)
		iconPathEntry.SetText(conf.IconPath)
	}
	execPickerBtn := widget.NewButton("选择...", func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			execPathEntry.SetText(reader.URI().Path())
			if nameEntry.Text == "" {
				fileName := filepath.Base(reader.URI().Path())
				nameEntry.SetText(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
			}
		}, t.win)
		d.Show()
	})
	iconPickerBtn := widget.NewButton("选择...", func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			iconPathEntry.SetText(reader.URI().Path())
		}, t.win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg", ".ico", ".svg"}))
		d.Show()
	})
	formItems := []*widget.FormItem{
		widget.NewFormItem("启动路径", container.NewBorder(nil, nil, nil, execPickerBtn, execPathEntry)),
		widget.NewFormItem("软件名称", nameEntry),
		widget.NewFormItem("软件图标", container.NewBorder(nil, nil, nil, iconPickerBtn, iconPathEntry)),
	}
	confirmDialog := dialog.NewForm(dialogTitle, "保存", "取消", formItems, func(ok bool) {
		if !ok {
			return
		}
		if execPathEntry.Text == "" || nameEntry.Text == "" {
			err := errors.New("启动路径和软件名称不能为空")
			dialog.ShowError(err, t.win)
			return
		}
		newConfig := ToolConfig{
			ExecPath: execPathEntry.Text,
			Name:     nameEntry.Text,
			IconPath: iconPathEntry.Text,
		}
		if index == -1 {
			t.configs = append(t.configs, newConfig)
		} else {
			t.configs[index] = newConfig
		}
		t.saveConfig()
		t.refreshUI()
	}, t.win)
	confirmDialog.Resize(fyne.NewSize(500, 250))
	confirmDialog.Show()
}
func (t *localLauncherTool) showRenameDialog(index int) {
	entry := widget.NewEntry()
	entry.SetText(t.configs[index].Name)
	dialog.ShowForm("重命名", "确认", "取消", []*widget.FormItem{widget.NewFormItem("新名称", entry)}, func(ok bool) {
		if !ok || entry.Text == "" {
			return
		}
		t.configs[index].Name = entry.Text
		t.saveConfig()
		t.refreshUI()
	}, t.win)
}
func (t *localLauncherTool) showChangeIconDialog(index int) {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		path := reader.URI().Path()
		t.configs[index].IconPath = path
		t.saveConfig()
		t.refreshUI()
	}, t.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg", ".ico", ".svg"}))
	d.Show()
}
func (t *localLauncherTool) showDeleteDialog(index int) {
	conf := t.configs[index]
	dialog.ShowConfirm("确认删除", "你确定要删除 '"+conf.Name+"' 吗？此操作无法撤销。", func(ok bool) {
		if !ok {
			return
		}
		t.configs = append(t.configs[:index], t.configs[index+1:]...)
		t.saveConfig()
		t.refreshUI()
	}, t.win)
}

// --- 辅助函数和类型 (无变化) ---
func (t *localLauncherTool) getIconResource(path string) fyne.Resource {
	if path == "" {
		return theme.ComputerIcon()
	}
	res, err := fyne.LoadResourceFromPath(path)
	if err != nil {
		log.Printf("加载图标 %s 失败: %v, 使用默认图标", path, err)
		return theme.ComputerIcon()
	}
	return res
}

type rightClickableButton struct {
	widget.Button
	OnTappedSecondary func(pe *fyne.PointEvent)
}

func newRightClickableButton(text string, icon fyne.Resource) *rightClickableButton {
	b := &rightClickableButton{}
	b.ExtendBaseWidget(b)
	b.SetIcon(icon)
	return b
}
func (b *rightClickableButton) TappedSecondary(pe *fyne.PointEvent) {
	if b.OnTappedSecondary != nil {
		b.OnTappedSecondary(pe)
	}
}
