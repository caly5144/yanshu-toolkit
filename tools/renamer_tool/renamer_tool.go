// 包名根据您的要求修改
package renamer_tool

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"yanshu-toolkit/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- 数据结构和规则实现 (无变化) ---
type Rule interface {
	Apply(string, int) string
	Describe() string
}
type RuleDefinition struct{ Type, Param1, Param2, Param3, Param4 string }
type FileItem struct{ OriginalPath, OriginalName, NewName, Status string }
type ReplaceRule struct{ Old, New string }

func (r *ReplaceRule) Apply(original string, index int) string {
	return strings.ReplaceAll(original, r.Old, r.New)
}
func (r *ReplaceRule) Describe() string { return fmt.Sprintf("替换: '%s' -> '%s'", r.Old, r.New) }

type InsertRule struct {
	Text     string
	Position int
}

func (r *InsertRule) Apply(original string, index int) string {
	if r.Position == -1 {
		return original + r.Text
	}
	return r.Text + original
}
func (r *InsertRule) Describe() string {
	posStr := "开头"
	if r.Position == -1 {
		posStr = "末尾"
	}
	return fmt.Sprintf("在%s插入: '%s'", posStr, r.Text)
}

type CaseRule struct{ CaseType string }

func (r *CaseRule) Apply(original string, index int) string {
	switch r.CaseType {
	case "upper":
		return strings.ToUpper(original)
	case "lower":
		return strings.ToLower(original)
	case "title":
		return strings.Title(strings.ToLower(original))
	}
	return original
}
func (r *CaseRule) Describe() string {
	desc := ""
	switch r.CaseType {
	case "upper":
		desc = "全部大写"
	case "lower":
		desc = "全部小写"
	case "title":
		desc = "首字母大写"
	}
	return fmt.Sprintf("大小写: %s", desc)
}

type SerializeRule struct{ Start, Step, Padding, Position int }

func (r *SerializeRule) Apply(original string, index int) string {
	num := r.Start + (index * r.Step)
	numStr := fmt.Sprintf(fmt.Sprintf("%%0%dd", r.Padding), num)
	if r.Position == -1 {
		return original + numStr
	}
	return numStr + original
}
func (r *SerializeRule) Describe() string {
	posStr := "开头"
	if r.Position == -1 {
		posStr = "末尾"
	}
	return fmt.Sprintf("在%s添加序列: 从%d开始, 步长%d, 补%d位", posStr, r.Start, r.Step, r.Padding)
}

// --- renamerTool 主结构 ---
func init() {
	core.Register(&renamerTool{})
}

type renamerTool struct {
	win      fyne.Window
	mainView fyne.CanvasObject

	// 核心变更: 放弃数据绑定，使用普通切片。移除所有锁。
	fileItems []*FileItem
	rules     []Rule

	// UI 组件引用
	ruleList          *widget.List
	previewList       *widget.List
	presetSelect      *widget.Select
	presets           map[string][]RuleDefinition
	selectedRuleIndex widget.ListItemID
}

func (t *renamerTool) Title() string       { return "批量重命名" }
func (t *renamerTool) Icon() fyne.Resource { return theme.DocumentCreateIcon() }
func (t *renamerTool) Category() string    { return "文件管理" }

// --- UI 构建 ---
func (t *renamerTool) View(win fyne.Window) fyne.CanvasObject {
	t.win = win
	t.fileItems = []*FileItem{}
	t.rules = []Rule{}
	t.selectedRuleIndex = -1

	t.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if t.mainView != nil && t.mainView.Visible() {
			t.addFilesFromURIs(uris)
		}
	})

	leftPanel := t.createLeftPanel()
	rightPanel := t.createRightPanel()
	bottomPanel := t.createBottomPanel()

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.4)

	t.mainView = container.NewBorder(nil, bottomPanel, nil, nil, split)
	return t.mainView
}

func (t *renamerTool) createLeftPanel() fyne.CanvasObject {
	t.ruleList = widget.NewList(
		func() int {
			return len(t.rules)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < len(t.rules) {
				o.(*widget.Label).SetText(t.rules[i].Describe())
			}
		},
	)
	t.ruleList.OnSelected = func(id widget.ListItemID) { t.selectedRuleIndex = id }
	t.ruleList.OnUnselected = func(id widget.ListItemID) {
		if t.selectedRuleIndex == id {
			t.selectedRuleIndex = -1
		}
	}
	addRuleBtn := widget.NewButtonWithIcon("添加规则", theme.ContentAddIcon(), func() { t.showAddRuleDialog() })
	removeRuleBtn := widget.NewButtonWithIcon("移除选中", theme.ContentRemoveIcon(), func() { t.removeSelectedRule() })
	t.presetSelect = widget.NewSelect([]string{}, func(name string) { t.loadPreset(name) })
	t.presetSelect.PlaceHolder = "加载预设..."
	t.loadPresets()
	savePresetBtn := widget.NewButtonWithIcon("保存", theme.DocumentSaveIcon(), func() { t.showSavePresetDialog() })
	deletePresetBtn := widget.NewButtonWithIcon("删除", theme.DeleteIcon(), t.deleteCurrentPreset)

	return container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("重命名规则", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), widget.NewLabelWithStyle("预设", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), t.presetSelect, container.NewGridWithColumns(2, savePresetBtn, deletePresetBtn)),
		nil, nil,
		container.NewBorder(nil, container.NewGridWithColumns(2, addRuleBtn, removeRuleBtn), nil, nil, t.ruleList),
	)
}

func (t *renamerTool) createRightPanel() fyne.CanvasObject {
	t.previewList = widget.NewList(
		func() int {
			return len(t.fileItems)
		},
		func() fyne.CanvasObject {
			return newColoredLabel()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < len(t.fileItems) {
				item := t.fileItems[i]
				o.(*coloredLabel).SetText(item.OriginalName, item.NewName, item.Status)
			}
		},
	)
	return container.NewBorder(widget.NewLabelWithStyle("预览 (可拖放文件到此窗口)", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), nil, nil, nil, t.previewList)
}

func (t *renamerTool) createBottomPanel() fyne.CanvasObject {
	addFilesBtn := widget.NewButton("选择文件...", func() { t.showSelectFilesDialog() })
	addFolderBtn := widget.NewButton("添加文件夹(递归)", func() { t.showAddFolderRecursiveDialog() })

	clearBtn := widget.NewButton("清空列表", func() {
		t.fileItems = []*FileItem{}
		t.previewList.Refresh()
		// 手动触发GC并建议将内存返回给操作系统
		runtime.GC()
		debug.FreeOSMemory()
	})

	renameBtn := widget.NewButtonWithIcon("开始重命名", theme.ConfirmIcon(), func() { t.executeRename() })
	renameBtn.Importance = widget.HighImportance
	return container.NewVBox(container.NewGridWithColumns(4, addFilesBtn, addFolderBtn, clearBtn, renameBtn), widget.NewSeparator())
}

// --- 文件处理方法 (回归原始同步逻辑) ---
func (t *renamerTool) addFile(path string) {
	t.fileItems = append(t.fileItems, &FileItem{OriginalPath: path, OriginalName: filepath.Base(path), NewName: filepath.Base(path)})
}

func (t *renamerTool) addFilesFromURIs(uris []fyne.URI) {
	var pathsToAdd []string
	for _, u := range uris {
		path := u.Path()
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					pathsToAdd = append(pathsToAdd, p)
				}
				return nil
			})
		} else {
			pathsToAdd = append(pathsToAdd, path)
		}
	}
	for _, p := range pathsToAdd {
		t.addFile(p)
	}
	t.updatePreviews()
}

func (t *renamerTool) showSelectFilesDialog() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		dirPath := uri.Path()
		files, err := ioutil.ReadDir(dirPath)
		if err != nil {
			dialog.ShowError(err, t.win)
			return
		}
		var fileNames []string
		var fileMap = make(map[string]string)
		for _, file := range files {
			if !file.IsDir() {
				fileNames = append(fileNames, file.Name())
				fileMap[file.Name()] = filepath.Join(dirPath, file.Name())
			}
		}
		if len(fileNames) == 0 {
			dialog.ShowInformation("提示", "该文件夹下没有文件。", t.win)
			return
		}
		checkGroup := widget.NewCheckGroup(fileNames, nil)
		selectAllBtn := widget.NewButton("全选/取消全选", func() {
			if len(checkGroup.Selected) == len(fileNames) {
				checkGroup.SetSelected(nil)
			} else {
				checkGroup.SetSelected(fileNames)
			}
		})
		content := container.NewBorder(selectAllBtn, nil, nil, nil, container.NewScroll(checkGroup))
		d := dialog.NewCustomConfirm("选择要添加的文件", "确定", "取消", content, func(ok bool) {
			if !ok {
				return
			}
			var uris []fyne.URI
			for _, name := range checkGroup.Selected {
				if fullPath, exists := fileMap[name]; exists {
					uris = append(uris, storage.NewFileURI(fullPath))
				}
			}
			t.addFilesFromURIs(uris)
		}, t.win)
		d.Resize(fyne.NewSize(400, 500))
		d.Show()
	}, t.win)
}

func (t *renamerTool) showAddFolderRecursiveDialog() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err == nil && uri != nil {
			t.addFilesFromURIs([]fyne.URI{uri})
		}
	}, t.win)
}

// --- 规则处理方法 ---
func (t *renamerTool) updatePreviews() {
	if len(t.fileItems) > 0 {
		for i, item := range t.fileItems {
			newName := item.OriginalName
			for _, rule := range t.rules {
				ext := filepath.Ext(newName)
				baseName := strings.TrimSuffix(newName, ext)
				baseName = rule.Apply(baseName, i)
				newName = baseName + ext
			}
			item.NewName = newName
			item.Status = ""
		}
	}
	t.previewList.Refresh()
}

func (t *renamerTool) addRule(rule Rule) {
	t.rules = append(t.rules, rule)
	t.ruleList.Refresh()
	t.updatePreviews()
}

func (t *renamerTool) removeSelectedRule() {
	idx := t.selectedRuleIndex
	if idx < 0 {
		return
	}
	t.rules = append(t.rules[:idx], t.rules[idx+1:]...)
	t.ruleList.UnselectAll()
	t.ruleList.Refresh()
	t.updatePreviews()
}

func (t *renamerTool) executeRename() {
	if len(t.fileItems) == 0 {
		dialog.ShowInformation("提示", "文件列表为空。", t.win)
		return
	}
	renamedCount, errorCount := 0, 0
	for i, item := range t.fileItems {
		if item.OriginalName == item.NewName {
			continue
		}
		newPath := filepath.Join(filepath.Dir(item.OriginalPath), item.NewName)
		if err := os.Rename(item.OriginalPath, newPath); err != nil {
			log.Printf("重命名失败: %s -> %s, 错误: %v", item.OriginalPath, newPath, err)
			item.Status = "error"
			errorCount++
		} else {
			item.Status = "success"
			item.OriginalPath = newPath
			item.OriginalName = item.NewName
			renamedCount++
		}
		t.previewList.RefreshItem(i)
	}
	dialog.ShowInformation("完成", fmt.Sprintf("重命名完成。\n成功: %d\n失败: %d", renamedCount, errorCount), t.win)
}

// --- Presets and Dialogs (无变化) ---
const presetsDir = "./data/renamer"

func (t *renamerTool) loadPresets() {
	if err := os.MkdirAll(presetsDir, 0755); err != nil {
		log.Printf("无法创建预设目录: %v", err)
		return
	}
	t.presets = make(map[string][]RuleDefinition)
	var presetNames []string
	files, err := ioutil.ReadDir(presetsDir)
	if err != nil {
		log.Printf("无法读取预设目录: %v", err)
		return
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			presetName := strings.TrimSuffix(file.Name(), ".json")
			filePath := filepath.Join(presetsDir, file.Name())
			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				continue
			}
			var defs []RuleDefinition
			if err := json.Unmarshal(data, &defs); err == nil {
				t.presets[presetName] = defs
				presetNames = append(presetNames, presetName)
			}
		}
	}
	sort.Strings(presetNames)
	t.presetSelect.Options = presetNames
	t.presetSelect.Refresh()
}

func (t *renamerTool) loadPreset(name string) {
	if defs, ok := t.presets[name]; ok {
		t.rules = []Rule{}
		for _, def := range defs {
			t.rules = append(t.rules, t.createRuleFromDef(def))
		}
		t.ruleList.Refresh()
		t.updatePreviews()
	}
}

func (t *renamerTool) showSavePresetDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("输入预设名称...")
	d := dialog.NewForm("保存预设", "确定", "取消", []*widget.FormItem{widget.NewFormItem("名称", entry)}, func(ok bool) {
		if ok && entry.Text != "" {
			if len(t.rules) == 0 {
				dialog.ShowInformation("提示", "没有可保存的规则。", t.win)
				return
			}
			var defs []RuleDefinition
			for _, r := range t.rules {
				defs = append(defs, t.createDefFromRule(r))
			}
			data, err := json.MarshalIndent(defs, "", "  ")
			if err != nil {
				dialog.ShowError(fmt.Errorf("无法序列化规则: %v", err), t.win)
				return
			}
			filePath := filepath.Join(presetsDir, entry.Text+".json")
			if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
				dialog.ShowError(fmt.Errorf("无法保存预设文件: %v", err), t.win)
				return
			}
			dialog.ShowInformation("成功", fmt.Sprintf("预设 '%s' 已保存。", entry.Text), t.win)
			t.loadPresets()
		}
	}, t.win)
	d.Resize(fyne.NewSize(300, 150))
	d.Show()
}

func (t *renamerTool) deleteCurrentPreset() {
	selected := t.presetSelect.Selected
	if selected == "" {
		dialog.ShowInformation("提示", "请先选择一个预设。", t.win)
		return
	}
	dialog.ShowConfirm("确认删除", fmt.Sprintf("确定要删除预设 '%s' 吗？", selected), func(confirm bool) {
		if !confirm {
			return
		}
		filePath := filepath.Join(presetsDir, selected+".json")
		if err := os.Remove(filePath); err != nil {
			dialog.ShowError(fmt.Errorf("删除失败: %v", err), t.win)
			return
		}
		t.presetSelect.ClearSelected()
		t.loadPresets()
	}, t.win)
}

func (t *renamerTool) createRuleFromDef(def RuleDefinition) Rule {
	switch def.Type {
	case "replace":
		return &ReplaceRule{Old: def.Param1, New: def.Param2}
	case "insert":
		pos, _ := strconv.Atoi(def.Param2)
		return &InsertRule{Text: def.Param1, Position: pos}
	case "case":
		return &CaseRule{CaseType: def.Param1}
	case "serialize":
		start, _ := strconv.Atoi(def.Param1)
		step, _ := strconv.Atoi(def.Param2)
		padding, _ := strconv.Atoi(def.Param3)
		pos, _ := strconv.Atoi(def.Param4)
		return &SerializeRule{Start: start, Step: step, Padding: padding, Position: pos}
	}
	return nil
}

func (t *renamerTool) createDefFromRule(rule Rule) RuleDefinition {
	switch r := rule.(type) {
	case *ReplaceRule:
		return RuleDefinition{Type: "replace", Param1: r.Old, Param2: r.New}
	case *InsertRule:
		return RuleDefinition{Type: "insert", Param1: r.Text, Param2: fmt.Sprintf("%d", r.Position)}
	case *CaseRule:
		return RuleDefinition{Type: "case", Param1: r.CaseType}
	case *SerializeRule:
		return RuleDefinition{Type: "serialize", Param1: fmt.Sprintf("%d", r.Start), Param2: fmt.Sprintf("%d", r.Step), Param3: fmt.Sprintf("%d", r.Padding), Param4: fmt.Sprintf("%d", r.Position)}
	}
	return RuleDefinition{}
}

func (t *renamerTool) showAddRuleDialog() {
	ruleTypes := []string{"插入", "替换", "大小写", "序列化"}
	var ruleGetters []func() Rule
	configStack := container.NewStack()
	for _, ruleType := range ruleTypes {
		var configUI fyne.CanvasObject
		var getter func() Rule
		switch ruleType {
		case "插入":
			textEntry := widget.NewEntry()
			textEntry.SetPlaceHolder("要插入的文本")
			posRadio := widget.NewRadioGroup([]string{"前缀", "后缀"}, nil)
			posRadio.SetSelected("前缀")
			configUI = container.NewVBox(widget.NewForm(widget.NewFormItem("插入:", textEntry)), widget.NewForm(widget.NewFormItem("位置:", posRadio)))
			getter = func() Rule {
				pos := 0
				if posRadio.Selected == "后缀" {
					pos = -1
				}
				return &InsertRule{Text: textEntry.Text, Position: pos}
			}
		case "替换":
			oldEntry := widget.NewEntry()
			oldEntry.SetPlaceHolder("要被替换的文本")
			newEntry := widget.NewEntry()
			newEntry.SetPlaceHolder("替换后的新文本")
			configUI = widget.NewForm(widget.NewFormItem("查找:", oldEntry), widget.NewFormItem("替换为:", newEntry))
			getter = func() Rule { return &ReplaceRule{Old: oldEntry.Text, New: newEntry.Text} }
		case "大小写":
			caseRadio := widget.NewRadioGroup([]string{"全部小写", "全部大写", "首字母大写"}, nil)
			caseRadio.SetSelected("全部小写")
			configUI = widget.NewForm(widget.NewFormItem("转换:", caseRadio))
			getter = func() Rule {
				caseType := "lower"
				if caseRadio.Selected == "全部大写" {
					caseType = "upper"
				} else if caseRadio.Selected == "首字母大写" {
					caseType = "title"
				}
				return &CaseRule{CaseType: caseType}
			}
		case "序列化":
			startEntry := widget.NewEntry()
			startEntry.SetText("1")
			stepEntry := widget.NewEntry()
			stepEntry.SetText("1")
			paddingEntry := widget.NewEntry()
			paddingEntry.SetText("2")
			posRadio := widget.NewRadioGroup([]string{"前缀", "后缀"}, nil)
			posRadio.SetSelected("前缀")
			configUI = container.NewVBox(widget.NewForm(widget.NewFormItem("起始数字:", startEntry), widget.NewFormItem("步长:", stepEntry), widget.NewFormItem("补零位数:", paddingEntry)), widget.NewForm(widget.NewFormItem("位置:", posRadio)))
			getter = func() Rule {
				start, _ := strconv.Atoi(startEntry.Text)
				step, _ := strconv.Atoi(stepEntry.Text)
				padding, _ := strconv.Atoi(paddingEntry.Text)
				pos := 0
				if posRadio.Selected == "后缀" {
					pos = -1
				}
				return &SerializeRule{Start: start, Step: step, Padding: padding, Position: pos}
			}
		}
		configStack.Add(configUI)
		configUI.Hide()
		ruleGetters = append(ruleGetters, getter)
	}
	var selectedIndex int
	typeList := widget.NewList(func() int { return len(ruleTypes) }, func() fyne.CanvasObject { return widget.NewLabel("") }, func(id widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(ruleTypes[id]) })
	typeList.OnSelected = func(id widget.ListItemID) {
		selectedIndex = id
		for i, obj := range configStack.Objects {
			if i == id {
				obj.Show()
			} else {
				obj.Hide()
			}
		}
		configStack.Refresh()
	}
	typeList.Select(0)
	content := container.NewHSplit(typeList, configStack)
	content.SetOffset(0.3)
	d := dialog.NewCustomConfirm("添加规则", "添加", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		newRule := ruleGetters[selectedIndex]()
		if newRule != nil {
			t.addRule(newRule)
		}
	}, t.win)
	d.Resize(fyne.NewSize(500, 300))
	d.Show()
}

type coloredLabel struct {
	widget.BaseWidget
	original, arrow, newPart *canvas.Text
	statusIcon               *widget.Icon
}

func newColoredLabel() *coloredLabel {
	c := &coloredLabel{original: canvas.NewText("", theme.ForegroundColor()), arrow: canvas.NewText("  ->  ", theme.ForegroundColor()), newPart: canvas.NewText("", color.NRGBA{R: 255, A: 255}), statusIcon: widget.NewIcon(nil)}
	c.ExtendBaseWidget(c)
	return c
}
func (c *coloredLabel) SetText(original, new, status string) {
	c.original.Text = original
	if original != new {
		c.arrow.Show()
		c.newPart.Show()
		c.newPart.Text = new
	} else {
		c.arrow.Hide()
		c.newPart.Hide()
	}
	if status == "success" {
		c.statusIcon.Show()
		c.statusIcon.SetResource(theme.ConfirmIcon())
	} else if status == "error" {
		c.statusIcon.Show()
		c.statusIcon.SetResource(theme.ErrorIcon())
	} else {
		c.statusIcon.Hide()
	}
	c.Refresh()
}
func (c *coloredLabel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewHBox(c.original, c.arrow, c.newPart, layout.NewSpacer(), c.statusIcon))
}
