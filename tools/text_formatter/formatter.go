package text_formatter

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"yanshu-toolkit/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/lascape/sat"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func New() core.Tool {
	return &textTool{}
}

func init() {
	core.RegisterFactory(New) // <-- 使用包名调用
}

func (t *textTool) Destroy() {
}

type textTool struct {
	lines         []string
	previousLines []string
	list          *widget.List
	win           fyne.Window
	undoBtn       *widget.Button
}

const maxDisplayLineLength = 200

// --- 自定义列表项 Widget 开始 ---
type listItemWidget struct {
	widget.BaseWidget
	label *widget.Label
}

func newListItemWidget() *listItemWidget {
	item := &listItemWidget{
		label: widget.NewLabel(""),
	}
	item.label.Wrapping = fyne.TextWrapWord
	item.ExtendBaseWidget(item)
	return item
}
func (i *listItemWidget) SetText(text string) {
	i.label.SetText(text)
	i.Refresh()
}
func (i *listItemWidget) CreateRenderer() fyne.WidgetRenderer {
	return &listItemRenderer{item: i}
}

type listItemRenderer struct {
	item *listItemWidget
}

func (r *listItemRenderer) MinSize() fyne.Size {
	return r.item.label.MinSize()
}
func (r *listItemRenderer) Layout(size fyne.Size) {
	r.item.label.Resize(size)
}
func (r *listItemRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.item.label}
}
func (r *listItemRenderer) Refresh() {
	r.item.label.Refresh()
}
func (r *listItemRenderer) Destroy() {}

// --- 自定义列表项 Widget 结束 ---

func (t *textTool) Title() string       { return "排版助手" }
func (t *textTool) Icon() fyne.Resource { return theme.DocumentIcon() }
func (t *textTool) Category() string    { return "文本工具" }

func (t *textTool) View(win fyne.Window) fyne.CanvasObject {
	t.win = win
	t.lines = []string{"在此处粘贴、输入或打开文本文件..."}

	t.list = widget.NewList(
		func() int {
			return len(t.lines)
		},
		func() fyne.CanvasObject {
			return newListItemWidget()
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			listItem := item.(*listItemWidget)
			listItem.SetText(t.lines[id])
			t.list.SetItemHeight(id, listItem.MinSize().Height)
		},
	)

	checkIndent := widget.NewCheck("段首缩进", nil)
	checkMergeLines := widget.NewCheck("合并换行", nil)
	checkDeleteBreaks := widget.NewCheck("删除非段落换行", nil)
	checkToSimplified := widget.NewCheck("转换为简体字", nil)
	checkCustomDict := widget.NewCheck("使用自定义字典", nil)

	// 【UI部分-1】创建新的UI组件
	checkSpacePara := widget.NewCheck("多个空格分隔段落", nil)
	entrySpaceCount := widget.NewEntry()
	entrySpaceCount.SetPlaceHolder("数量")
	entrySpaceCount.SetText("4") // 默认值
	entrySpaceCount.Disable()    // 默认禁用

	checkSpacePara.OnChanged = func(checked bool) {
		if checked {
			entrySpaceCount.Enable()
		} else {
			entrySpaceCount.Disable()
		}
	}

	formatOptions := container.NewHBox(
		checkIndent, checkMergeLines, checkDeleteBreaks, checkToSimplified, checkCustomDict,
		// 【UI部分-2】将新组件添加到布局中
		checkSpacePara, entrySpaceCount,
	)

	executeBtn := widget.NewButtonWithIcon("执行", theme.ConfirmIcon(), func() {
		if len(t.lines) == 0 {
			return
		}

		// 【UI部分-3】执行前的输入验证
		var spaceSplitCount int
		if checkSpacePara.Checked {
			var err error
			spaceSplitCount, err = strconv.Atoi(entrySpaceCount.Text)
			if err != nil || spaceSplitCount < 2 {
				dialog.ShowError(errors.New("空格数量必须是大于等于2的数字"), win)
				return
			}
		}

		if checkCustomDict.Checked {
			dictPath := filepath.Join("data", "custom_dict.txt")
			if _, err := os.Stat(dictPath); os.IsNotExist(err) {
				dialog.ShowError(errors.New("自定义字典文件未找到: "+dictPath), win)
				return
			}
		}
		t.previousLines = make([]string, len(t.lines))
		copy(t.previousLines, t.lines)
		progress := dialog.NewProgressInfinite("正在处理", "请稍候...", win)
		progress.Show()
		go func() {
			var selectedOptions []string
			if checkIndent.Checked {
				selectedOptions = append(selectedOptions, "段首缩进")
			}
			if checkMergeLines.Checked {
				selectedOptions = append(selectedOptions, "合并换行")
			}
			if checkDeleteBreaks.Checked {
				selectedOptions = append(selectedOptions, "删除非段落换行")
			}
			if checkToSimplified.Checked {
				selectedOptions = append(selectedOptions, "转换为简体字")
			}
			if checkCustomDict.Checked {
				selectedOptions = append(selectedOptions, "使用自定义字典")
			}
			// 【UI部分-4】将新选项和其值传递给后台
			if checkSpacePara.Checked {
				selectedOptions = append(selectedOptions, "多个空格分隔段落", strconv.Itoa(spaceSplitCount))
			}

			rebuiltText := strings.Builder{}
			isPrevLineParaBreak := true
			for _, line := range t.lines {
				if line == "" {
					isPrevLineParaBreak = true
					continue
				}
				if !isPrevLineParaBreak {
					rebuiltText.WriteString(line)
				} else {
					if rebuiltText.Len() > 0 {
						rebuiltText.WriteString("\n\n")
					}
					rebuiltText.WriteString(line)
				}
				isPrevLineParaBreak = false
			}
			textForFormatting := rebuiltText.String()

			processedText := formatTextStream(strings.NewReader(textForFormatting), selectedOptions)
			rawProcessedLines := strings.Split(processedText, "\n")
			var finalDisplayLines []string
			for _, line := range rawProcessedLines {
				finalDisplayLines = append(finalDisplayLines, splitLineForDisplay(line, maxDisplayLineLength)...)
			}

			fyne.Do(func() {
				t.lines = finalDisplayLines
				t.list.Refresh()
				t.undoBtn.Enable()
				progress.Hide()
			})
		}()
	})

	t.undoBtn = widget.NewButtonWithIcon("撤销", theme.ContentUndoIcon(), func() {
		if t.previousLines == nil {
			return
		}
		t.lines = t.previousLines
		t.previousLines = nil
		t.list.Refresh()
		t.undoBtn.Disable()
	})
	t.undoBtn.Disable()

	openBtn := widget.NewButtonWithIcon("打开", theme.FileIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			if reader == nil {
				return // 用户取消
			}

			progress := dialog.NewProgressInfinite("正在读取", "正在解析文件内容...", win)
			progress.Show()

			go func() {
				defer reader.Close()

				content, readErr := io.ReadAll(reader) // 1. 先读取原始字节
				if readErr != nil {
					fyne.Do(func() {
						progress.Hide()
						dialog.ShowError(readErr, win)
					})
					return
				}

				utf8Content := decodeToUTF8(content)

				text := strings.ReplaceAll(string(utf8Content), "\r\n", "\n")
				rawLines := strings.Split(text, "\n")

				var processedLines []string
				for _, line := range rawLines {
					processedLines = append(processedLines, splitLineForDisplay(line, maxDisplayLineLength)...)
				}

				fyne.Do(func() {
					progress.Hide()
					if len(processedLines) == 0 {
						t.lines = []string{"文件为空。"}
					} else {
						t.lines = processedLines
					}
					t.previousLines = nil
					t.undoBtn.Disable()
					t.list.Refresh()
				})
			}()
		}, win)
		txtFilter := storage.NewExtensionFileFilter([]string{".txt"})
		fileDialog.SetFilter(txtFilter)
		fileDialog.Show()
	})

	copyBtn := widget.NewButtonWithIcon("复制全文", theme.ContentCopyIcon(), func() {
		rebuiltText := strings.Builder{}
		isPrevLineParaBreak := true
		for _, line := range t.lines {
			if line == "" {
				isPrevLineParaBreak = true
				continue
			}
			if !isPrevLineParaBreak {
				rebuiltText.WriteString(line)
			} else {
				if rebuiltText.Len() > 0 {
					rebuiltText.WriteString("\n\n")
				}
				rebuiltText.WriteString(line)
			}
			isPrevLineParaBreak = false
		}
		win.Clipboard().SetContent(rebuiltText.String())
	})

	pasteBtn := widget.NewButtonWithIcon("粘贴并替换", theme.ContentPasteIcon(), func() {
		content := win.Clipboard().Content()
		if content == "" {
			return
		}
		progress := dialog.NewProgressInfinite("正在处理", "正在解析粘贴的文本...", win)
		progress.Show()
		go func() {
			rawLines := strings.Split(content, "\n")
			var processedLines []string
			for _, line := range rawLines {
				processedLines = append(processedLines, splitLineForDisplay(line, maxDisplayLineLength)...)
			}
			fyne.Do(func() {
				t.lines = processedLines
				t.previousLines = nil
				t.undoBtn.Disable()
				t.list.Refresh()
				progress.Hide()
			})
		}()
	})

	clearBtn := widget.NewButtonWithIcon("清空", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("确认", "确定要清空所有文本吗？", func(confirm bool) {
			if confirm {
				t.lines = nil
				t.previousLines = nil
				t.lines = []string{"内容已清空。"}

				t.undoBtn.Disable()
				t.list.Refresh()
			}
		}, win)
	})

	buttonToolbar := container.NewGridWithColumns(6, executeBtn, t.undoBtn, openBtn, copyBtn, pasteBtn, clearBtn)
	topControls := container.NewVBox(buttonToolbar, container.New(layout.NewCenterLayout(), formatOptions), widget.NewSeparator())

	return container.NewBorder(topControls, nil, nil, nil, t.list)
}

// 【核心修改】实现带优先级的分割逻辑
func splitLineForDisplay(line string, maxLength int) []string {
	runes := []rune(line)
	if len(runes) <= maxLength {
		return []string{line}
	}

	var result []string
	var current int
	for current < len(runes) {
		// 如果剩余部分已经不长，则直接作为最后一行并结束
		if len(runes)-current <= maxLength {
			result = append(result, string(runes[current:]))
			break
		}

		end := current + maxLength
		breakPoint := -1

		// 为了避免产生过短的行，我们只在行的后半部分寻找断点
		searchStart := current + maxLength/2
		if searchStart >= end {
			searchStart = current + 1
		}

		// --- 优先级 1: 寻找空格 (半角、全角、制表符) ---
		for i := end - 1; i >= searchStart; i-- {
			switch runes[i] {
			case ' ', '\t', '　':
				breakPoint = i + 1
				goto foundBreakpoint // 找到最高优先级的，直接跳转
			}
		}

		// --- 优先级 2: 寻找句号 (中英文) ---
		// (只有在没找到空格时才会执行到这里)
		for i := end - 1; i >= searchStart; i-- {
			switch runes[i] {
			case '.', '。':
				breakPoint = i + 1
				goto foundBreakpoint // 找到第二优先级的，跳转
			}
		}

		// --- 优先级 3: 寻找其他常见标点 (回退方案) ---
		// (只有在上述都找不到时才会执行)
		for i := end - 1; i >= searchStart; i-- {
			switch runes[i] {
			case ',', '，', '!', '！', '?', '？', ';', '；', ':', '：':
				breakPoint = i + 1
				goto foundBreakpoint // 找到其他标点，跳转
			}
		}

	foundBreakpoint: // 所有查找逻辑的出口
		if breakPoint != -1 {
			// 如果找到了任何一个断点，就用它
			result = append(result, string(runes[current:breakPoint]))
			current = breakPoint
		} else {
			// 实在找不到任何合适的断点，执行硬分割
			result = append(result, string(runes[current:end]))
			current = end
		}
	}
	return result
}

var reParagraphSeparator = regexp.MustCompile(`\n{2,}`)

func applyCustomDictionary(text string, dictPath string) (string, error) {
	file, err := os.Open(dictPath)
	if err != nil {
		return text, err
	}
	defer file.Close()
	var replacerArgs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := regexp.MustCompile(`[\t ]+`).Split(line, 2)
		if len(parts) == 2 {
			oldWord, newWord := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if oldWord != "" {
				replacerArgs = append(replacerArgs, oldWord, newWord)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return text, err
	}
	if len(replacerArgs) == 0 {
		return text, nil
	}
	replacer := strings.NewReplacer(replacerArgs...)
	return replacer.Replace(text), nil
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// 如果已经是UTF-8或者转换失败，它会返回原始数据var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// decodeToUTF8 自动检测并移除BOM，然后尝试从GBK转换为UTF-8
func decodeToUTF8(data []byte) []byte {
	// 1. 检查并移除UTF-8 BOM
	if bytes.HasPrefix(data, utf8BOM) {
		// 如果文件以BOM开头，说明它肯定是UTF-8编码。
		// 我们只需去掉BOM，然后直接返回剩余部分即可。
		return bytes.TrimPrefix(data, utf8BOM)
	}

	// 2. 如果没有BOM，再尝试进行GBK解码（逻辑和之前一样）
	// 使用 GB18030 解码器，因为它是 GBK 和 GB2312 的超集，兼容性最好
	decoder := simplifiedchinese.GB18030.NewDecoder()

	utf8Data, _, err := transform.Bytes(decoder, data)
	if err != nil {
		// 转换失败，说明它很可能本来就是（不带BOM的）UTF-8编码
		return data
	}

	// 转换成功，返回转换后的UTF-8数据
	return utf8Data
}

// 【核心逻辑部分】修改 formatTextStream 函数
func formatTextStream(reader io.Reader, options []string) string {
	opts := make(map[string]bool)
	var spaceSplitCount int

	// 解析选项，注意要处理新选项的值
	for i := 0; i < len(options); i++ {
		opt := options[i]
		if opt == "多个空格分隔段落" {
			// 如果找到了这个选项，它的值就在下一个位置
			if i+1 < len(options) {
				count, err := strconv.Atoi(options[i+1])
				if err == nil {
					spaceSplitCount = count
					opts[opt] = true
					i++ // 跳过值，避免下次循环处理
				}
			}
		} else {
			opts[opt] = true
		}
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		log.Println("读取流失败:", err)
		return ""
	}
	text := string(content)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 【核心逻辑部分-1】最高优先级处理：多个空格分段
	if opts["多个空格分隔段落"] && spaceSplitCount >= 2 {
		// 动态构建正则表达式，匹配N个或更多的空格/制表符
		pattern := fmt.Sprintf(`[ \t　]{%d,}`, spaceSplitCount)
		re := regexp.MustCompile(pattern)
		// 将匹配到的多个空格替换为标准段落分隔符
		text = re.ReplaceAllString(text, "\n\n")
	}

	if opts["使用自定义字典"] {
		dictPath := filepath.Join("data", "custom_dict.txt")
		var replaceErr error
		text, replaceErr = applyCustomDictionary(text, dictPath)
		if replaceErr != nil {
			log.Printf("应用自定义字典时出错: %v", replaceErr)
		}
	}
	if opts["转换为简体字"] {
		dicter := sat.DefaultDict()
		text = dicter.Read(text)
	}

	// 【核心逻辑部分-2】后续处理现在基于可能已被空格分段的文本
	paragraphs := reParagraphSeparator.Split(text, -1)
	var processedParagraphs []string
	for _, p := range paragraphs {
		currentParagraph := p
		if opts["合并换行"] || opts["删除非段落换行"] {
			currentParagraph = strings.ReplaceAll(currentParagraph, "\n", "")
		}
		trimmedParagraph := strings.TrimSpace(currentParagraph)
		if trimmedParagraph == "" {
			continue
		}
		if opts["合并换行"] {
			fields := strings.Fields(trimmedParagraph)
			trimmedParagraph = strings.Join(fields, " ")
		}
		if opts["段首缩进"] {
			processedParagraphs = append(processedParagraphs, "　　"+trimmedParagraph)
		} else {
			processedParagraphs = append(processedParagraphs, trimmedParagraph)
		}
	}
	return strings.Join(processedParagraphs, "\n\n")
}
