package locallauncher

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	defaultGroup   = "未分组"
)

type ToolType string

const (
	AppTool ToolType = "app"
	WebTool ToolType = "web"
)

type ToolConfig struct {
	Name  string   `json:"name"`
	Type  ToolType `json:"type"`
	Path  string   `json:"path"`
	Icon  string   `json:"icon"`
	Group string   `json:"group"`
}

type SaveData struct {
	Configs    []ToolConfig `json:"configs"`
	GroupOrder []string     `json:"group_order"`
}

type localLauncherTool struct {
	configs    []ToolConfig
	groupOrder []string
	win        fyne.Window
	container  *fyne.Container
}

func init() {
	core.Register(&localLauncherTool{})
}

func (t *localLauncherTool) Title() string       { return "本地启动器" }
func (t *localLauncherTool) Icon() fyne.Resource { return theme.HomeIcon() }
func (t *localLauncherTool) Category() string    { return "常用工具" }

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
			t.configs = []ToolConfig{}
			t.groupOrder = []string{}
		} else {
			log.Printf("读取配置文件失败: %v", err)
		}
		return
	}

	var saveData SaveData
	err = json.Unmarshal(data, &saveData)
	if err != nil {
		var oldConfigs []ToolConfig
		if json.Unmarshal(data, &oldConfigs) == nil {
			log.Println("检测到旧版配置文件，正在迁移...")
			t.configs = oldConfigs
			t.buildGroupOrderFromConfigs()
			t.saveConfig()
			return
		}
		log.Printf("解析配置文件失败: %v", err)
		t.configs = []ToolConfig{}
		t.groupOrder = []string{}
		return
	}

	t.configs = saveData.Configs
	t.groupOrder = saveData.GroupOrder
	t.cleanupAndValidateData()
}

func (t *localLauncherTool) buildGroupOrderFromConfigs() {
	groups := make(map[string]bool)
	for _, conf := range t.configs {
		groupName := conf.Group
		if groupName == "" {
			groupName = defaultGroup
		}
		groups[groupName] = true
	}

	var orderedGroups []string
	for g := range groups {
		if g != defaultGroup {
			orderedGroups = append(orderedGroups, g)
		}
	}
	sort.Strings(orderedGroups)
	if groups[defaultGroup] || len(t.configs) > 0 || len(orderedGroups) > 0 {
		t.groupOrder = append(orderedGroups, defaultGroup)
	} else {
		t.groupOrder = orderedGroups
	}
}

func (t *localLauncherTool) cleanupAndValidateData() {
	configGroups := make(map[string]bool)
	for i := range t.configs {
		if t.configs[i].Group == "" {
			t.configs[i].Group = defaultGroup
		}
		configGroups[t.configs[i].Group] = true
	}

	orderGroups := make(map[string]bool)
	var newOrder []string
	hasDefaultInOrder := false
	for _, g := range t.groupOrder {
		if g == defaultGroup {
			hasDefaultInOrder = true
		}

		// FIX: Keep the group if it has configs OR if it is the special default group.
		if _, ok := configGroups[g]; ok || g == defaultGroup {
			if !orderGroups[g] {
				newOrder = append(newOrder, g)
				orderGroups[g] = true
			}
		}
	}

	// 确保所有在configs里的组都存在于order里
	for g := range configGroups {
		if _, ok := orderGroups[g]; !ok {
			newOrder = append(newOrder, g)
		}
	}

	// 确保 "未分组" 总是存在并且在最后
	var finalOrder []string
	if !hasDefaultInOrder {
		finalOrder = newOrder
		finalOrder = append(finalOrder, defaultGroup)
	} else {
		for _, g := range newOrder {
			if g != defaultGroup {
				finalOrder = append(finalOrder, g)
			}
		}
		finalOrder = append(finalOrder, defaultGroup)
	}

	t.groupOrder = finalOrder
}

func (t *localLauncherTool) saveConfig() {
	path := t.configPath()
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
	}
	saveData := SaveData{Configs: t.configs, GroupOrder: t.groupOrder}
	data, err := json.MarshalIndent(saveData, "", "  ")
	if err != nil {
		log.Printf("序列化配置失败: %v", err)
		return
	}
	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		log.Printf("写入配置文件失败: %v", err)
	}
}

// --- UI 构建 ---
func (t *localLauncherTool) refreshUI() {
	t.cleanupAndValidateData() // Refresh UI時也清理一下数据保证同步
	if len(t.configs) == 0 {
		t.container.Objects = []fyne.CanvasObject{t.createEmptyView()}
	} else {
		t.container.Objects = []fyne.CanvasObject{t.createGridView()}
	}
	t.container.Refresh()
}

func (t *localLauncherTool) createEmptyView() fyne.CanvasObject {
	return container.NewCenter(widget.NewButtonWithIcon("添加第一个", theme.ContentAddIcon(), func() {
		t.showEditDialog(-1)
	}))
}

func (t *localLauncherTool) createGridView() fyne.CanvasObject {
	groupsMap := make(map[string][]int)
	for i, conf := range t.configs {
		groupsMap[conf.Group] = append(groupsMap[conf.Group], i)
	}

	allGroupsContent := container.NewVBox()
	for i, groupName := range t.groupOrder {
		titleLabel := widget.NewLabel(groupName)
		titleLabel.TextStyle.Bold = true

		upBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
			if i > 0 {
				t.groupOrder[i], t.groupOrder[i-1] = t.groupOrder[i-1], t.groupOrder[i]
				t.saveConfig()
				t.refreshUI()
			}
		})
		downBtn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
			if i < len(t.groupOrder)-2 {
				t.groupOrder[i], t.groupOrder[i+1] = t.groupOrder[i+1], t.groupOrder[i]
				t.saveConfig()
				t.refreshUI()
			}
		})

		if i == 0 || groupName == defaultGroup {
			upBtn.Disable()
		}
		if i >= len(t.groupOrder)-1 || groupName == defaultGroup {
			downBtn.Disable()
		}

		var titleBar fyne.CanvasObject
		if groupName == defaultGroup {
			titleBar = container.NewHBox(titleLabel)
		} else {
			titleBar = container.NewHBox(titleLabel, layout.NewSpacer(), upBtn, downBtn)
		}

		indices := groupsMap[groupName]
		gridContent := make([]fyne.CanvasObject, 0, len(indices)+1)
		if indices != nil {
			for _, index := range indices {
				gridContent = append(gridContent, t.createToolButton(index))
			}
		}

		if groupName == defaultGroup {
			addBtnContainer := container.NewVBox(
				widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() { t.showEditDialog(-1) }),
				widget.NewLabelWithStyle("添加", fyne.TextAlignCenter, fyne.TextStyle{}),
			)
			gridContent = append(gridContent, addBtnContainer)
		}

		// 即使是空的分组（除了未分组），也显示一个空的卡片，避免界面跳动
		var content fyne.CanvasObject = container.NewGridWrap(fyne.NewSize(100, 100), gridContent...)
		if len(gridContent) == 0 {
			content = widget.NewLabelWithStyle("（空）", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
		}
		contentCard := widget.NewCard("", "", content)

		allGroupsContent.Add(container.NewVBox(titleBar, contentCard))
	}

	return container.NewScroll(allGroupsContent)
}

func (t *localLauncherTool) createToolButton(index int) fyne.CanvasObject {
	conf := t.configs[index]
	toolBtn := newRightClickableButton(conf.Name, t.getIconResource(conf))
	toolBtn.OnTapped = func() {
		log.Printf("正在启动: %s (%s)", conf.Path, conf.Type)
		var err error
		switch conf.Type {
		case AppTool:
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd", "/C", "start", "", conf.Path)
			} else {
				cmd = exec.Command(conf.Path)
				cmd.Dir = filepath.Dir(conf.Path)
			}
			err = cmd.Start()
		case WebTool:
			u, parseErr := url.Parse(conf.Path)
			if parseErr != nil {
				err = parseErr
				break
			}
			err = fyne.CurrentApp().OpenURL(u)
		default:
			err = errors.New("未知的工具类型: " + string(conf.Type))
		}
		if err != nil {
			log.Printf("启动 %s 失败: %v", conf.Name, err)
			dialog.ShowError(err, t.win)
		}
	}
	toolBtn.OnTappedSecondary = func(pe *fyne.PointEvent) {
		editItem := fyne.NewMenuItem("编辑", func() { t.showEditDialog(index) })
		deleteItem := fyne.NewMenuItem("删除", func() { t.showDeleteDialog(index) })
		popup := widget.NewPopUpMenu(fyne.NewMenu("", editItem, fyne.NewMenuItemSeparator(), deleteItem), t.win.Canvas())
		popup.ShowAtPosition(pe.AbsolutePosition)
	}

	nameLabel := widget.NewLabel(conf.Name)
	nameLabel.Wrapping = fyne.TextWrapWord
	nameLabel.Alignment = fyne.TextAlignCenter
	return container.NewVBox(toolBtn, nameLabel)
}

func (t *localLauncherTool) showEditDialog(index int) {
	pathEntry := widget.NewEntry()
	nameEntry := widget.NewEntry()
	iconEntry := widget.NewEntry()
	groupEntry := widget.NewEntry()
	groupEntry.SetPlaceHolder("例如: 开发工具, 娱乐...")

	pathPickerBtn := widget.NewButton("选择...", func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			pathEntry.SetText(reader.URI().Path())
			if nameEntry.Text == "" {
				nameEntry.SetText(strings.TrimSuffix(filepath.Base(reader.URI().Path()), filepath.Ext(reader.URI().Path())))
			}
		}, t.win)
		d.Show()
	})
	pathContainer := container.NewBorder(nil, nil, nil, pathPickerBtn, pathEntry)

	typeSelect := widget.NewSelect([]string{"本地软件", "网站"}, func(s string) {
		if s == "本地软件" {
			pathEntry.SetPlaceHolder("点击右侧按钮选择可执行文件...")
			nameEntry.SetPlaceHolder("软件名称")
			pathPickerBtn.Show()
		} else {
			pathEntry.SetPlaceHolder("请输入完整的网址, 例如 https://www.google.com")
			nameEntry.SetPlaceHolder("网站名称")
			pathPickerBtn.Hide()
			if nameEntry.Text == "" {
				nameEntry.SetText("新网站")
			}
		}
		pathContainer.Refresh()
	})

	iconPickerBtn := widget.NewButton("选择...", func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			iconEntry.SetText(reader.URI().Path())
		}, t.win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg", ".ico", ".svg"}))
		d.Show()
	})

	dialogTitle := "添加新条目"
	if index >= 0 {
		conf := t.configs[index]
		dialogTitle = "编辑条目"
		pathEntry.SetText(conf.Path)
		nameEntry.SetText(conf.Name)
		iconEntry.SetText(conf.Icon)
		groupEntry.SetText(conf.Group)
		if conf.Type == WebTool {
			typeSelect.SetSelected("网站")
		} else {
			typeSelect.SetSelected("本地软件")
		}
	} else {
		typeSelect.SetSelected("本地软件")
	}

	formItems := []*widget.FormItem{
		widget.NewFormItem("类型", typeSelect),
		widget.NewFormItem("分组名称", groupEntry),
		widget.NewFormItem("路径/网址", pathContainer),
		widget.NewFormItem("名称", nameEntry),
		widget.NewFormItem("图标", container.NewBorder(nil, nil, nil, iconPickerBtn, iconEntry)),
	}

	confirmDialog := dialog.NewForm(dialogTitle, "保存", "取消", formItems, func(ok bool) {
		if !ok {
			return
		}
		if pathEntry.Text == "" || nameEntry.Text == "" {
			dialog.ShowError(errors.New("路径/网址和名称不能为空"), t.win)
			return
		}

		var toolType ToolType
		if typeSelect.Selected == "网站" {
			toolType = WebTool
		} else {
			toolType = AppTool
		}

		groupName := groupEntry.Text
		if groupName == "" {
			groupName = defaultGroup
		}

		isNewGroup := true
		for _, g := range t.groupOrder {
			if g == groupName {
				isNewGroup = false
				break
			}
		}

		if isNewGroup && groupName != defaultGroup {
			if len(t.groupOrder) <= 1 {
				t.groupOrder = []string{groupName, defaultGroup}
			} else {
				lastGroupIdx := len(t.groupOrder) - 1
				t.groupOrder = append(t.groupOrder[:lastGroupIdx], groupName, t.groupOrder[lastGroupIdx])
			}
		}

		newConfig := ToolConfig{Type: toolType, Path: pathEntry.Text, Name: nameEntry.Text, Icon: iconEntry.Text, Group: groupName}
		if index == -1 {
			t.configs = append(t.configs, newConfig)
		} else {
			oldGroup := t.configs[index].Group
			t.configs[index] = newConfig
			// Check if the old group is now empty
			isOldGroupEmpty := true
			for _, c := range t.configs {
				if c.Group == oldGroup {
					isOldGroupEmpty = false
					break
				}
			}
			if isOldGroupEmpty {
				t.cleanupAndValidateData()
			}
		}

		t.saveConfig()
		t.refreshUI()
	}, t.win)
	confirmDialog.Resize(fyne.NewSize(500, 300))
	confirmDialog.Show()
}

func (t *localLauncherTool) showDeleteDialog(index int) {
	conf := t.configs[index]
	dialog.ShowConfirm("确认删除", "你确定要删除 '"+conf.Name+"' 吗？此操作无法撤销。", func(ok bool) {
		if !ok {
			return
		}
		groupOfDeletedItem := t.configs[index].Group
		t.configs = append(t.configs[:index], t.configs[index+1:]...)

		isGroupEmpty := true
		for _, c := range t.configs {
			if c.Group == groupOfDeletedItem {
				isGroupEmpty = false
				break
			}
		}
		if isGroupEmpty && groupOfDeletedItem != defaultGroup {
			var newOrder []string
			for _, g := range t.groupOrder {
				if g != groupOfDeletedItem {
					newOrder = append(newOrder, g)
				}
			}
			t.groupOrder = newOrder
		}
		t.saveConfig()
		t.refreshUI()
	}, t.win)
}

func (t *localLauncherTool) getIconResource(conf ToolConfig) fyne.Resource {
	if conf.Icon != "" {
		res, err := fyne.LoadResourceFromPath(conf.Icon)
		if err == nil {
			return res
		}
		log.Printf("加载图标 %s 失败: %v, 使用默认图标", conf.Icon, err)
	}
	if conf.Type == WebTool {
		return theme.SearchIcon()
	}
	return theme.ComputerIcon()
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
