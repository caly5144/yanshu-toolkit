package image_browser

import (
	"fmt"
	"image"
	"io/fs"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"yanshu-toolkit/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/image/draw"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	core.Register(&imageBrowserTool{})
}

// --- 自定义高性能图片控件 (无变化) ---
type scalableImage struct {
	widget.BaseWidget
	mu   sync.RWMutex
	img  image.Image
	path string
	tool *imageBrowserTool
}

func newScalableImage(tool *imageBrowserTool) *scalableImage {
	s := &scalableImage{tool: tool}
	s.ExtendBaseWidget(s)
	return s
}
func (s *scalableImage) SetImage(img image.Image, path string) {
	s.mu.Lock()
	s.img = img
	s.path = path
	s.mu.Unlock()
	s.Refresh()
}
func (s *scalableImage) Tapped(*fyne.PointEvent) {}
func (s *scalableImage) TappedSecondary(e *fyne.PointEvent) {
	s.mu.RLock()
	path := s.path
	s.mu.RUnlock()
	if path == "" {
		return
	}
	showInExplorer := fyne.NewMenuItem("在文件浏览器中显示", func() {
		if err := open.Run(path); err != nil {
			if err2 := open.Start(filepath.Dir(path)); err2 != nil {
				dialog.ShowError(err2, s.tool.parentWin)
			}
		}
	})
	copyPathItem := fyne.NewMenuItem("复制路径", func() {
		s.tool.parentWin.Clipboard().SetContent(path)
		s.tool.updateStatus("路径已复制。", false)
	})
	deleteItem := fyne.NewMenuItem("删除文件", func() {
		dialog.ShowConfirm("确认删除", fmt.Sprintf("确定要永久删除文件吗？\n%s", path), func(ok bool) {
			if !ok {
				return
			}
			go func() {
				err := os.Remove(path)
				fyne.Do(func() {
					if err != nil {
						dialog.ShowError(err, s.tool.parentWin)
						s.tool.updateStatus("删除失败。", true)
						return
					}
					s.tool.updateStatus("文件已删除。", false)
					for i, p := range s.tool.imagePaths {
						if p == path {
							s.tool.imagePaths = append(s.tool.imagePaths[:i], s.tool.imagePaths[i+1:]...)
							break
						}
					}
					s.tool.showNextImage()
				})
			}()
		}, s.tool.parentWin)
	})
	propItem := fyne.NewMenuItem("属性", func() {
		info, err := os.Stat(path)
		if err != nil {
			dialog.ShowError(err, s.tool.parentWin)
			return
		}
		sizeMB := float64(info.Size()) / 1024 / 1024
		content := fmt.Sprintf("文件名: %s\n大小: %.2f MB\n修改时间: %s", info.Name(), sizeMB, info.ModTime().Format("2006-01-02 15:04:05"))
		dialog.ShowInformation("文件属性", content, s.tool.parentWin)
	})
	menu := fyne.NewMenu("", showInExplorer, copyPathItem, fyne.NewMenuItemSeparator(), deleteItem, propItem)
	widget.ShowPopUpMenuAtPosition(menu, s.tool.parentWin.Canvas(), e.AbsolutePosition)
}
func (s *scalableImage) CreateRenderer() fyne.WidgetRenderer {
	renderer := &scalableImageRenderer{scalable: s}
	raster := canvas.NewRaster(renderer.draw)
	renderer.raster = raster
	return renderer
}

type scalableImageRenderer struct {
	scalable *scalableImage
	raster   *canvas.Raster
}

func (r *scalableImageRenderer) draw(w, h int) image.Image {
	r.scalable.mu.RLock()
	img := r.scalable.img
	r.scalable.mu.RUnlock()
	if img == nil || w < 1 || h < 1 {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}
	srcSize := img.Bounds().Size()
	ratioSrc := float32(srcSize.X) / float32(srcSize.Y)
	ratioDst := float32(w) / float32(h)
	var newWidth, newHeight int
	if ratioSrc > ratioDst {
		newWidth = w
		newHeight = int(float32(w) / ratioSrc)
	} else {
		newHeight = h
		newWidth = int(float32(h) * ratioSrc)
	}
	if newWidth < 1 || newHeight < 1 {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	x0 := (w - newWidth) / 2
	y0 := (h - newHeight) / 2
	dstRect := image.Rect(x0, y0, x0+newWidth, y0+newHeight)
	draw.CatmullRom.Scale(dst, dstRect, img, img.Bounds(), draw.Over, nil)
	return dst
}
func (r *scalableImageRenderer) Layout(size fyne.Size)        { r.raster.Resize(size) }
func (r *scalableImageRenderer) MinSize() fyne.Size           { return fyne.NewSize(50, 50) }
func (r *scalableImageRenderer) Refresh()                     { r.raster.Refresh() }
func (r *scalableImageRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.raster} }
func (r *scalableImageRenderer) Destroy()                     {}

// --- 主程序 ---
const (
	PlayModeRandom = "乱序播放"
	PlayModeOrder  = "顺序播放"
)

type imageBrowserTool struct {
	view               fyne.CanvasObject
	parentWin          fyne.Window
	folderLabel        *widget.Label
	folderSelectBtn    *widget.Button
	intervalEntry      *widget.Entry
	includeSubdir      *widget.Check
	extensionsEntry    *widget.Entry
	statusLabel        *widget.Label
	startButton        *widget.Button
	prevButton         *widget.Button
	nextButton         *widget.Button
	clearButton        *widget.Button
	fullscreenBtn      *widget.Button
	toggleConfigBtn    *widget.Button
	configForm         *widget.Form
	imageHostContainer fyne.CanvasObject
	displayWidget      *scalableImage
	dropHint           fyne.CanvasObject
	dirAccordion       *widget.Accordion
	contentSplit       *container.Split
	imagePaths         []string
	imagePathsByDir    map[string][]string
	orderedDirKeys     []string
	selectedFolder     string
	ticker             *time.Ticker
	isRunning          bool
	playModeSelect     *widget.Select
	currentPlayMode    string
	currentImageIndex  int
	fullscreenWin      fyne.Window
	fullscreenImage    *scalableImage
}

func (t *imageBrowserTool) Title() string       { return "图片浏览器" }
func (t *imageBrowserTool) Icon() fyne.Resource { return theme.FileImageIcon() }
func (t *imageBrowserTool) Category() string    { return "媒体工具" }

func (t *imageBrowserTool) View(win fyne.Window) fyne.CanvasObject {
	if t.view != nil {
		return t.view
	}
	t.parentWin = win
	t.parentWin.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			t.handleDrop(uris[0])
		}
	})
	t.displayWidget = newScalableImage(t)
	t.folderLabel = widget.NewLabel("点击右侧按钮选择文件夹...")
	t.folderLabel.Wrapping = fyne.TextTruncate
	t.folderSelectBtn = widget.NewButton("选择...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			t.handleNewFolder(uri.Path())
		}, t.parentWin)
	})
	t.intervalEntry = widget.NewEntry()
	t.intervalEntry.SetText("5")
	t.extensionsEntry = widget.NewEntry()
	t.extensionsEntry.SetText(".jpg,.jpeg,.png,.gif,.bmp,.webp")
	t.currentPlayMode = PlayModeRandom
	t.playModeSelect = widget.NewSelect([]string{PlayModeRandom, PlayModeOrder}, nil)
	t.playModeSelect.SetSelected(PlayModeRandom)
	t.includeSubdir = widget.NewCheck("扫描子文件夹", nil)
	t.includeSubdir.SetChecked(true)
	hintLabel := widget.NewLabel("拖放文件夹到此处开始\n或点击上方“选择...”按钮")
	hintLabel.Alignment = fyne.TextAlignCenter
	hintLabel.Wrapping = fyne.TextWrapWord
	t.dropHint = container.New(layout.NewCenterLayout(), hintLabel)
	t.dirAccordion = widget.NewAccordion()
	t.dirAccordion.Hide()
	t.statusLabel = widget.NewLabel("请先选择一个文件夹。")
	t.statusLabel.Alignment = fyne.TextAlignLeading
	t.startButton = widget.NewButtonWithIcon("开始", theme.MediaPlayIcon(), t.toggle)
	t.prevButton = widget.NewButtonWithIcon("上一张", theme.MediaSkipPreviousIcon(), t.showPrevImage)
	t.nextButton = widget.NewButtonWithIcon("下一张", theme.MediaSkipNextIcon(), t.showNextImage)
	t.fullscreenBtn = widget.NewButtonWithIcon("全屏", theme.ViewFullScreenIcon(), t.toggleFullscreen)
	t.clearButton = widget.NewButtonWithIcon("清除", theme.DeleteIcon(), t.clearAll)

	// **修正：使用可靠的图标，并设置初始文本**
	t.toggleConfigBtn = widget.NewButtonWithIcon("收起设置", theme.MenuExpandIcon(), t.toggleConfigPanel)
	t.toggleConfigBtn.Importance = widget.LowImportance

	folderSelector := container.NewBorder(nil, nil, nil, t.folderSelectBtn, t.folderLabel)
	optionsBox := container.NewHBox(t.playModeSelect, t.includeSubdir)
	t.configForm = widget.NewForm(widget.NewFormItem("图片文件夹", folderSelector), widget.NewFormItem("刷新间隔(秒)", t.intervalEntry), widget.NewFormItem("", optionsBox), widget.NewFormItem("图片类型", t.extensionsEntry))

	t.imageHostContainer = container.NewStack(t.displayWidget, t.dropHint)
	scrollableAccordion := container.NewScroll(t.dirAccordion)
	t.contentSplit = container.NewHSplit(t.imageHostContainer, scrollableAccordion)
	t.prevButton.Disable()
	t.nextButton.Disable()
	t.fullscreenBtn.Disable()
	t.clearButton.Disable()

	// **修正：将切换按钮放回底部控制栏**
	controlBar := container.NewHBox(t.startButton, t.prevButton, t.nextButton, t.clearButton, t.fullscreenBtn, layout.NewSpacer(), t.toggleConfigBtn, t.statusLabel)

	t.view = container.NewBorder(t.configForm, controlBar, nil, nil, t.contentSplit)

	t.playModeSelect.OnChanged = func(s string) {
		t.currentPlayMode = s
		t.resetForNewScan()
		t.updateStatus("播放模式已切换，请重新开始。", false)
	}
	t.includeSubdir.OnChanged = func(b bool) { t.resetForNewScan(); t.updateStatus("扫描选项已切换，请重新开始。", false) }

	t.updateDisplayState()
	t.updateDirAccordion()
	return t.view
}

// **修正：使用 Hide/Show 方法和可靠的图标**
func (t *imageBrowserTool) toggleConfigPanel() {
	if t.configForm.Visible() {
		t.configForm.Hide()
		t.toggleConfigBtn.SetIcon(theme.MenuDropDownIcon())
		t.toggleConfigBtn.SetText("展开设置")
	} else {
		t.configForm.Show()
		t.toggleConfigBtn.SetIcon(theme.MenuExpandIcon())
		t.toggleConfigBtn.SetText("收起设置")
	}
}
func (t *imageBrowserTool) toggle() {
	if t.isRunning {
		if t.ticker != nil {
			t.ticker.Stop()
		}
		t.isRunning = false
		t.startButton.SetText("开始")
		t.startButton.SetIcon(theme.MediaPlayIcon())
		t.setFormEnabled(true)
		if len(t.imagePaths) > 0 {
			t.prevButton.Enable()
			t.nextButton.Enable()
		} else {
			t.prevButton.Disable()
			t.nextButton.Disable()
		}
		t.updateStatus(fmt.Sprintf("已暂停。共 %d 张图片。", len(t.imagePaths)), false)
	} else {
		if t.selectedFolder == "" {
			t.updateStatus("错误: 请先选择一个文件夹。", true)
			return
		}
		if len(t.imagePaths) == 0 {
			t.updateStatus("正在扫描图片...", false)
			go func() {
				scanErr := t.scanImages()
				fyne.Do(func() {
					if scanErr != nil {
						t.updateStatus(fmt.Sprintf("扫描失败: %v", scanErr), true)
						return
					}
					if len(t.imagePaths) == 0 {
						t.updateStatus("未在该文件夹下找到任何支持的图片。", true)
						return
					}
					t.startPlayback()
				})
			}()
		} else {
			t.startPlayback()
		}
	}
}
func (t *imageBrowserTool) startPlayback() {
	intervalSec, err := strconv.Atoi(t.intervalEntry.Text)
	if err != nil || intervalSec <= 0 {
		intervalSec = 5
		t.intervalEntry.SetText("5")
	}
	t.updateStatus(fmt.Sprintf("播放中... 共 %d 张图片。", len(t.imagePaths)), false)
	t.setFormEnabled(false)
	t.isRunning = true
	t.startButton.SetText("暂停")
	t.startButton.SetIcon(theme.MediaPauseIcon())
	t.prevButton.Enable()
	t.nextButton.Enable()
	t.fullscreenBtn.Enable()
	t.clearButton.Enable()
	fyne.Do(t.updateDirAccordion)
	t.displayWidget.mu.RLock()
	hasImage := t.displayWidget.img != nil
	t.displayWidget.mu.RUnlock()
	if !hasImage {
		if t.currentPlayMode == PlayModeOrder {
			t.showImageAtIndex(0)
		} else {
			t.showRandomImage()
		}
	}
	t.ticker = time.NewTicker(time.Duration(intervalSec) * time.Second)
	go func() {
		for range t.ticker.C {
			if !t.isRunning {
				return
			}
			fyne.Do(t.showNextImage)
		}
	}()
}
func (t *imageBrowserTool) scanImages() error {
	rootPath := t.selectedFolder
	includeSubdir := t.includeSubdir.Checked
	extStr := strings.ToLower(t.extensionsEntry.Text)
	allowedExts := make(map[string]bool)
	for _, ext := range strings.Split(extStr, ",") {
		allowedExts[strings.TrimSpace(ext)] = true
	}
	t.imagePaths = []string{}
	t.imagePathsByDir = make(map[string][]string)
	t.orderedDirKeys = []string{}
	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("访问路径 %s 时出错: %v", path, err)
			return nil
		}
		if !d.IsDir() {
			if allowedExts[strings.ToLower(filepath.Ext(path))] {
				t.imagePaths = append(t.imagePaths, path)
				if t.currentPlayMode == PlayModeOrder {
					dir := filepath.Dir(path)
					t.imagePathsByDir[dir] = append(t.imagePathsByDir[dir], path)
				}
			}
		}
		return nil
	}
	if includeSubdir {
		filepath.WalkDir(rootPath, walkFunc)
	} else {
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			info, _ := entry.Info()
			walkFunc(filepath.Join(rootPath, entry.Name()), fs.FileInfoToDirEntry(info), nil)
		}
	}
	if t.currentPlayMode == PlayModeOrder {
		sort.Strings(t.imagePaths)
		for dir, paths := range t.imagePathsByDir {
			sort.Strings(paths)
			t.imagePathsByDir[dir] = paths
			t.orderedDirKeys = append(t.orderedDirKeys, dir)
		}
		sort.Strings(t.orderedDirKeys)
	}
	t.currentImageIndex = -1
	return nil
}
func (t *imageBrowserTool) showNextImage() {
	if len(t.imagePaths) == 0 {
		return
	}
	if t.currentPlayMode == PlayModeRandom {
		t.showRandomImage()
	} else {
		t.currentImageIndex++
		if t.currentImageIndex >= len(t.imagePaths) {
			t.currentImageIndex = 0
		}
		t.showImageAtIndex(t.currentImageIndex)
	}
}
func (t *imageBrowserTool) showPrevImage() {
	if len(t.imagePaths) == 0 {
		return
	}
	if t.currentPlayMode == PlayModeRandom {
		t.showRandomImage()
	} else {
		t.currentImageIndex--
		if t.currentImageIndex < 0 {
			t.currentImageIndex = len(t.imagePaths) - 1
		}
		t.showImageAtIndex(t.currentImageIndex)
	}
}
func (t *imageBrowserTool) showRandomImage() {
	if len(t.imagePaths) == 0 {
		return
	}
	randomIndex := rand.Intn(len(t.imagePaths))
	t.showImageAtIndex(randomIndex)
}
func (t *imageBrowserTool) showImageAtIndex(index int) {
	if index < 0 || index >= len(t.imagePaths) {
		t.displayWidget.SetImage(nil, "")
		t.updateDisplayState()
		t.updateStatus("没有更多图片了。", false)
		return
	}
	t.currentImageIndex = index
	path := t.imagePaths[index]
	file, err := os.Open(path)
	if err != nil {
		t.updateStatus(fmt.Sprintf("无法打开: %v", err), true)
		return
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.updateStatus(fmt.Sprintf("无法解码: %v", err), true)
		return
	}
	t.displayWidget.SetImage(img, path)
	if t.fullscreenImage != nil {
		t.fullscreenImage.SetImage(img, path)
	}
	t.updateDisplayState()
	if t.currentPlayMode == PlayModeOrder {
		t.updateStatus(fmt.Sprintf("播放中... (%d / %d)", t.currentImageIndex+1, len(t.imagePaths)), false)
	}
}
func (t *imageBrowserTool) clearAll() {
	if t.isRunning {
		t.toggle()
	}
	t.selectedFolder = ""
	t.folderLabel.SetText("点击右侧按钮选择文件夹...")
	t.resetForNewScan()
	t.updateStatus("已清除，请重新选择文件夹并开始。", false)
}
func (t *imageBrowserTool) resetForNewScan() {
	if t.isRunning {
		t.toggle()
	}
	t.displayWidget.SetImage(nil, "")
	if t.fullscreenImage != nil {
		t.fullscreenImage.SetImage(nil, "")
	}
	t.imagePaths = []string{}
	t.imagePathsByDir = nil
	t.orderedDirKeys = nil
	t.prevButton.Disable()
	t.nextButton.Disable()
	t.fullscreenBtn.Disable()
	t.clearButton.Disable()
	t.updateDirAccordion()
	t.updateDisplayState()
}
func (t *imageBrowserTool) handleDrop(uri fyne.URI) {
	path := uri.Path()
	info, err := os.Stat(path)
	if err != nil {
		t.updateStatus(fmt.Sprintf("无法访问拖放路径: %v", err), true)
		return
	}
	if !info.IsDir() {
		path = filepath.Dir(path)
	}
	t.handleNewFolder(path)
}
func (t *imageBrowserTool) handleNewFolder(path string) {
	t.resetForNewScan()
	t.selectedFolder = path
	t.folderLabel.SetText(path)
	t.updateStatus("文件夹已更新，请点击“开始”播放。", false)
}
func (t *imageBrowserTool) updateDisplayState() {
	if t.displayWidget == nil || t.dropHint == nil || t.imageHostContainer == nil {
		return
	}
	t.displayWidget.mu.RLock()
	hasImage := t.displayWidget.img != nil
	t.displayWidget.mu.RUnlock()
	if hasImage {
		t.dropHint.Hide()
	} else {
		t.dropHint.Show()
	}
	if container, ok := t.imageHostContainer.(*fyne.Container); ok {
		container.Refresh()
	}
}
func (t *imageBrowserTool) updateDirAccordion() {
	if t.contentSplit == nil {
		return
	}
	showAccordion := t.currentPlayMode == PlayModeOrder && t.includeSubdir.Checked && len(t.imagePathsByDir) > 1
	if showAccordion {
		t.dirAccordion.Items = []*widget.AccordionItem{}
		tree := make(map[string]map[string]bool)
		basePathLen := len(t.selectedFolder) + 1
		for _, dir := range t.orderedDirKeys {
			if len(dir) < basePathLen {
				continue
			}
			relPath := dir[basePathLen:]
			parts := strings.SplitN(relPath, string(os.PathSeparator), 2)
			if len(parts) > 0 {
				level1 := parts[0]
				if _, ok := tree[level1]; !ok {
					tree[level1] = make(map[string]bool)
				}
				if len(parts) > 1 && parts[1] != "" {
					tree[level1][parts[1]] = true
				}
			}
		}
		var topLevelKeys []string
		for k := range tree {
			topLevelKeys = append(topLevelKeys, k)
		}
		sort.Strings(topLevelKeys)
		for _, level1Key := range topLevelKeys {
			subItemsMap := tree[level1Key]
			if len(subItemsMap) == 0 {
				btn := widget.NewButton(level1Key, t.createJumpToAction(filepath.Join(t.selectedFolder, level1Key)))
				item := widget.NewAccordionItem(level1Key, btn)
				t.dirAccordion.Append(item)
			} else {
				vbox := container.NewVBox()
				var subKeys []string
				for k := range subItemsMap {
					subKeys = append(subKeys, k)
				}
				sort.Strings(subKeys)
				for _, subKey := range subKeys {
					fullSubPath := filepath.Join(t.selectedFolder, level1Key, subKey)
					displayText := "    " + filepath.Base(subKey)
					url, _ := url.Parse("")
					link := widget.NewHyperlink(displayText, url)
					link.OnTapped = t.createJumpToAction(fullSubPath)
					vbox.Add(link)
				}
				item := widget.NewAccordionItem(level1Key, vbox)
				t.dirAccordion.Append(item)
			}
		}
		t.dirAccordion.Show()
		t.contentSplit.SetOffset(0.8)
	} else {
		t.dirAccordion.Hide()
		t.contentSplit.SetOffset(1.0)
	}
	t.contentSplit.Refresh()
}
func (t *imageBrowserTool) createJumpToAction(path string) func() {
	return func() {
		for i, p := range t.imagePaths {
			if strings.HasPrefix(p, path) {
				t.showImageAtIndex(i)
				break
			}
		}
	}
}
func (t *imageBrowserTool) toggleFullscreen() {
	if t.fullscreenWin != nil {
		t.fullscreenWin.Close()
		return
	}
	t.displayWidget.mu.RLock()
	img := t.displayWidget.img
	path := t.displayWidget.path
	t.displayWidget.mu.RUnlock()
	if img == nil {
		return
	}
	if t.fullscreenImage == nil {
		t.fullscreenImage = newScalableImage(t)
	}
	t.fullscreenImage.SetImage(img, path)
	win := fyne.CurrentApp().NewWindow("全屏图片查看 (按 ESC 退出)")
	t.fullscreenWin = win
	win.SetOnClosed(func() { t.fullscreenWin = nil })
	win.SetContent(t.fullscreenImage)
	win.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		if key.Name == fyne.KeyEscape {
			win.Close()
		}
	})
	win.SetFullScreen(true)
	win.Show()
}
func (t *imageBrowserTool) updateStatus(msg string, isError bool) {
	if t.statusLabel == nil {
		return
	}
	if isError {
		t.statusLabel.SetText("错误: " + msg)
	} else {
		t.statusLabel.SetText(msg)
	}
}
func (t *imageBrowserTool) setFormEnabled(enabled bool) {
	if t.configForm == nil || t.folderSelectBtn == nil {
		return
	}
	if enabled {
		t.configForm.Enable()
		t.folderSelectBtn.Enable()
	} else {
		t.configForm.Disable()
		t.folderSelectBtn.Disable()
	}
}
