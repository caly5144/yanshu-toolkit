package image_browser

import (
	"fmt"
	"image"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"

	"yanshu-toolkit/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	// **最终修正**: 修正了致命的拼写错误
	"github.com/skratchdot/open-golang/open"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	core.Register(&imageBrowserTool{})
}

// --- 自定义高性能图片控件 (最终现实版) ---
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
					s.tool.showRandomImage()
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

// ... CreateRenderer 和 renderer 的代码保持不变 ...
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
	nextButton         *widget.Button
	clearButton        *widget.Button
	fullscreenBtn      *widget.Button
	configForm         *widget.Form
	imageHostContainer *fyne.Container
	displayWidget      *scalableImage
	imagePaths         []string
	selectedFolder     string
	ticker             *time.Ticker
	isRunning          bool
	fullscreenWin      fyne.Window
	fullscreenImage    *scalableImage
	dropHint           fyne.CanvasObject
}

func (t *imageBrowserTool) Title() string       { return "图片随机浏览器" }
func (t *imageBrowserTool) Icon() fyne.Resource { return theme.FileImageIcon() }
func (t *imageBrowserTool) Category() string    { return "媒体工具" }
func (t *imageBrowserTool) View(win fyne.Window) fyne.CanvasObject {
	if t.view != nil {
		return t.view
	}
	t.parentWin = win
	t.parentWin.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			// 我们只处理第一个拖放的项目
			t.handleDrop(uris[0])
		}
	})
	if len(fyne.CurrentApp().Driver().AllWindows()) > 0 {
		t.parentWin = fyne.CurrentApp().Driver().AllWindows()[0]
	}
	t.folderLabel = widget.NewLabel("点击右侧按钮选择文件夹...")
	t.folderLabel.Wrapping = fyne.TextTruncate
	t.folderSelectBtn = widget.NewButton("选择...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			t.selectedFolder = path
			t.folderLabel.SetText(path)
			t.updateStatus("文件夹已选择，请点击开始。", false)
		}, t.parentWin)
	})
	folderSelector := container.NewBorder(nil, nil, nil, t.folderSelectBtn, t.folderLabel)
	t.intervalEntry = widget.NewEntry()
	t.intervalEntry.SetText("5")
	t.includeSubdir = widget.NewCheck("扫描子文件夹", nil)
	t.includeSubdir.SetChecked(true)
	t.extensionsEntry = widget.NewEntry()
	t.extensionsEntry.SetText(".jpg,.jpeg,.png,.gif,.bmp,.webp")
	t.configForm = widget.NewForm(widget.NewFormItem("图片文件夹", folderSelector), widget.NewFormItem("刷新间隔(秒)", t.intervalEntry), widget.NewFormItem("包含子文件夹", t.includeSubdir), widget.NewFormItem("图片类型", t.extensionsEntry))

	t.displayWidget = newScalableImage(t)

	hintLabel := widget.NewLabel("拖放文件夹到此处开始")
	hintLabel.Alignment = fyne.TextAlignCenter
	hintLabel.Wrapping = fyne.TextWrapWord
	t.dropHint = container.New(layout.NewCenterLayout(), hintLabel)

	t.imageHostContainer = container.NewStack(t.displayWidget, t.dropHint)
	t.statusLabel = widget.NewLabel("请先选择一个文件夹。")
	t.statusLabel.Alignment = fyne.TextAlignLeading
	t.startButton = widget.NewButtonWithIcon("开始", theme.MediaPlayIcon(), t.toggle)
	t.nextButton = widget.NewButtonWithIcon("下一张", theme.MediaSkipNextIcon(), func() {
		if len(t.imagePaths) > 0 {
			t.showRandomImage()
		}
	})
	t.fullscreenBtn = widget.NewButtonWithIcon("全屏", theme.ViewFullScreenIcon(), t.toggleFullscreen)
	t.clearButton = widget.NewButtonWithIcon("清除", theme.DeleteIcon(), func() {
		if t.isRunning {
			t.toggle() // 先暂停
		}
		t.displayWidget.SetImage(nil, "")
		// 如果全屏窗口存在，也清理它
		if t.fullscreenImage != nil {
			t.fullscreenImage.SetImage(nil, "")
		}
		t.imagePaths = []string{}
		t.selectedFolder = ""
		t.folderLabel.SetText("点击右侧按钮选择文件夹...")
		t.nextButton.Disable()
		t.fullscreenBtn.Disable()
		t.clearButton.Disable()
		t.updateStatus("已清除，请重新选择文件夹并开始。", false)
		t.updateDisplayState() // <--- 关键调用
	})
	t.nextButton.Disable()
	t.fullscreenBtn.Disable()
	t.clearButton.Disable()
	controlBar := container.NewHBox(t.startButton, t.nextButton, t.clearButton, t.fullscreenBtn, layout.NewSpacer(), t.statusLabel)
	t.view = container.NewBorder(t.configForm, controlBar, nil, nil, t.imageHostContainer)
	t.updateDisplayState()
	return t.view
}

// in func (t *imageBrowserTool) handleDrop(uri fyne.URI)
func (t *imageBrowserTool) handleDrop(uri fyne.URI) {
	path := uri.Path()
	info, err := os.Stat(path)
	if err != nil {
		t.updateStatus(fmt.Sprintf("无法访问拖放路径: %v", err), true)
		return
	}

	// 如果拖入的是文件，则使用其父目录
	if !info.IsDir() {
		path = filepath.Dir(path)
	}

	// 如果正在播放，先暂停。这部分保留是合理的，因为换了目录，旧的播放应该停止。
	if t.isRunning {
		t.toggle() // 调用toggle来暂停
	}

	// 清理旧的图片路径列表和显示的图片，因为目录已经变了
	t.imagePaths = []string{}
	t.displayWidget.SetImage(nil, "")
	if t.fullscreenImage != nil {
		t.fullscreenImage.SetImage(nil, "")
	}
	t.updateDisplayState() // 清空图片后，确保提示显示出来

	// 设置新文件夹并更新UI
	t.selectedFolder = path
	t.folderLabel.SetText(path)

	// 更新状态提示，但不自动开始
	t.updateStatus("文件夹已通过拖放更新，请点击“开始”播放。", false)

	// 自动播放
	// t.toggle() // <-- 注释或删除这一行
}

// New helper function for imageBrowserTool
func (t *imageBrowserTool) updateDisplayState() {
	t.displayWidget.mu.RLock()
	hasImage := t.displayWidget.img != nil
	t.displayWidget.mu.RUnlock()

	if hasImage {
		t.dropHint.Hide()
	} else {
		t.dropHint.Show()
	}
	t.imageHostContainer.Refresh()
}

func (t *imageBrowserTool) toggle() {
	if t.isRunning {
		if t.ticker != nil {
			t.ticker.Stop()
		}
		// 如果全屏窗口存在，则关闭它
		if t.fullscreenWin != nil {
			t.fullscreenWin.Close()
		}
		t.isRunning = false
		t.startButton.SetText("开始")
		t.startButton.SetIcon(theme.MediaPlayIcon())
		t.setFormEnabled(true)
		t.updateStatus(fmt.Sprintf("已暂停。共 %d 张图片。", len(t.imagePaths)), false)
	} else {
		if t.selectedFolder == "" {
			t.updateStatus("请先选择一个文件夹。", true)
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
		intervalSec = 10
	}
	t.updateStatus(fmt.Sprintf("播放中... 共 %d 张图片。", len(t.imagePaths)), false)
	t.setFormEnabled(false)
	t.isRunning = true
	t.startButton.SetText("暂停")
	t.startButton.SetIcon(theme.MediaPauseIcon())
	t.nextButton.Enable()
	t.fullscreenBtn.Enable()
	t.clearButton.Enable()
	t.displayWidget.mu.RLock()
	img := t.displayWidget.img
	t.displayWidget.mu.RUnlock()
	if img == nil {
		t.showRandomImage()
	}
	t.ticker = time.NewTicker(time.Duration(intervalSec) * time.Second)
	go func() {
		for range t.ticker.C {
			if !t.isRunning {
				return
			}
			fyne.Do(t.showRandomImage)
		}
	}()
}
func (t *imageBrowserTool) showRandomImage() {
	if len(t.imagePaths) == 0 {
		// 清理主窗口的图片
		t.displayWidget.SetImage(nil, "")
		// 如果全屏窗口存在，也清理它
		if t.fullscreenImage != nil {
			t.fullscreenImage.SetImage(nil, "")
		}
		t.updateStatus("没有更多图片了。", false)
		t.updateDisplayState()
		return
	}

	randomIndex := rand.Intn(len(t.imagePaths))
	randomPath := t.imagePaths[randomIndex]

	file, err := os.Open(randomPath)
	if err != nil {
		// 处理错误...（为了简洁省略）
		t.updateStatus(fmt.Sprintf("无法打开图片: %v", err), true)
		// 从列表中移除有问题的图片，避免重复失败
		t.imagePaths = append(t.imagePaths[:randomIndex], t.imagePaths[randomIndex+1:]...)
		go fyne.Do(t.showRandomImage) // 尝试下一张
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		// 处理错误...（为了简洁省略）
		t.updateStatus(fmt.Sprintf("无法解码图片: %v", err), true)
		t.imagePaths = append(t.imagePaths[:randomIndex], t.imagePaths[randomIndex+1:]...)
		go fyne.Do(t.showRandomImage) // 尝试下一张
		return
	}

	// **核心修改：同时更新两个控件**
	t.displayWidget.SetImage(img, randomPath)
	if t.fullscreenImage != nil {
		t.fullscreenImage.SetImage(img, randomPath)
	}
	t.updateDisplayState()
}
func (t *imageBrowserTool) toggleFullscreen() {
	// 如果全屏窗口已存在，则关闭它
	if t.fullscreenWin != nil {
		t.fullscreenWin.Close()
		return
	}

	// 如果没有图片，则不进入全屏
	t.displayWidget.mu.RLock()
	img := t.displayWidget.img
	path := t.displayWidget.path
	t.displayWidget.mu.RUnlock()
	if img == nil {
		return
	}

	// 创建一个新的 scalableImage 用于全屏显示
	// 注意这里也传入了 t，这样右键菜单等功能依然能正常工作
	t.fullscreenImage = newScalableImage(t)
	t.fullscreenImage.SetImage(img, path) // 使用当前图片初始化

	// 创建新窗口
	win := fyne.CurrentApp().NewWindow("全屏图片查看 (按 ESC 退出)")
	t.fullscreenWin = win

	win.SetOnClosed(func() {
		t.fullscreenWin = nil
		t.fullscreenImage = nil
	})

	// 设置内容为新的全屏图片控件
	win.SetContent(t.fullscreenImage)

	// 绑定ESC退出
	win.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		if key.Name == fyne.KeyEscape {
			win.Close()
		}
	})

	win.SetFullScreen(true)
	win.Show()
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
	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("访问路径 %s 时出错: %v", path, err)
			return nil
		}
		if !d.IsDir() {
			if allowedExts[strings.ToLower(filepath.Ext(path))] {
				t.imagePaths = append(t.imagePaths, path)
			}
		}
		return nil
	}
	if includeSubdir {
		return filepath.WalkDir(rootPath, walkFunc)
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, _ := entry.Info()
		walkFunc(filepath.Join(rootPath, entry.Name()), fs.FileInfoToDirEntry(info), nil)
	}
	return nil
}
func (t *imageBrowserTool) updateStatus(msg string, isError bool) {
	if isError {
		t.statusLabel.SetText("错误: " + msg)
	} else {
		t.statusLabel.SetText(msg)
	}
}
func (t *imageBrowserTool) setFormEnabled(enabled bool) {
	if enabled {
		t.configForm.Enable()
		t.folderSelectBtn.Enable()
	} else {
		t.configForm.Disable()
		t.folderSelectBtn.Disable()
	}
}
