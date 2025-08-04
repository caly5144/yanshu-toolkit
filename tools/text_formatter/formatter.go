package text_formatter

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
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
)

func init() { core.Register(&textTool{}) }

type textTool struct {
	lines         []string
	previousLines []string
	list          *widget.List
	undoBtn       *widget.Button
	win           fyne.Window
}

func (t *textTool) Title() string       { return "排版助手" }
func (t *textTool) Icon() fyne.Resource { return theme.DocumentIcon() }
func (t *textTool) Category() string    { return "文本工具" }

func (t *textTool) View(win fyne.Window) fyne.CanvasObject {
	t.win = win
	t.lines = []string{"在此处粘贴、输入或打开文本文件..."}

	t.list = widget.NewList(
		func() int { return len(t.lines) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("template")
			label.Wrapping = fyne.TextWrapWord
			return label
		},
		func(id widget.ListItemID, item fyne.CanvasObject) { item.(*widget.Label).SetText(t.lines[id]) },
	)

	checkIndent := widget.NewCheck("段首缩进", nil)
	checkMergeLines := widget.NewCheck("合并换行", nil)
	checkDeleteBreaks := widget.NewCheck("删除非段落换行", nil)
	checkToSimplified := widget.NewCheck("转换为简体字", nil)
	checkCustomDict := widget.NewCheck("使用自定义字典", nil)
	// checkCustomDict.Disable()

	checkToSimplified.OnChanged = func(checked bool) {
		if checked {
			checkCustomDict.Enable()
		} else {
			checkCustomDict.SetChecked(false)
			checkCustomDict.Disable()
		}
	}

	formatOptions := container.NewHBox(
		checkIndent, checkMergeLines, checkDeleteBreaks, checkToSimplified, checkCustomDict,
	)

	executeBtn := widget.NewButtonWithIcon("执行", theme.ConfirmIcon(), func() {
		if len(t.lines) == 0 {
			return
		}

		if checkToSimplified.Checked && checkCustomDict.Checked {
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

			processedText := formatTextStream(strings.NewReader(strings.Join(t.lines, "\n")), selectedOptions)

			fyne.Do(func() {
				t.lines = strings.Split(processedText, "\n")
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
				return
			}
			defer reader.Close()

			var newLines []string
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				newLines = append(newLines, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				dialog.ShowError(err, win)
				return
			}

			t.lines = newLines
			t.previousLines = nil
			t.undoBtn.Disable()
			t.list.Refresh()
		}, win)

		txtFilter := storage.NewExtensionFileFilter([]string{".txt"})
		fileDialog.SetFilter(txtFilter)
		fileDialog.Show()
	})

	copyBtn := widget.NewButtonWithIcon("复制全文", theme.ContentCopyIcon(), func() {
		win.Clipboard().SetContent(strings.Join(t.lines, "\n"))
	})
	pasteBtn := widget.NewButtonWithIcon("粘贴并替换", theme.ContentPasteIcon(), func() {
		content := win.Clipboard().Content()
		t.lines = strings.Split(content, "\n")
		t.previousLines = nil
		t.undoBtn.Disable()
		t.list.Refresh()
	})
	clearBtn := widget.NewButtonWithIcon("清空", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("确认", "确定要清空所有文本吗？", func(confirm bool) {
			if confirm {
				t.lines = []string{}
				t.previousLines = nil
				t.undoBtn.Disable()
				t.list.Refresh()
			}
		}, win)
	})

	buttonToolbar := container.NewGridWithColumns(6, executeBtn, t.undoBtn, openBtn, copyBtn, pasteBtn, clearBtn)
	topControls := container.NewVBox(buttonToolbar, container.New(layout.NewCenterLayout(), formatOptions), widget.NewSeparator())
	return container.NewBorder(topControls, nil, nil, nil, t.list)
}

// applyCustomDictionary 使用高性能的 strings.Replacer 从自定义字典文件对文本进行批量替换
func applyCustomDictionary(text string, dictPath string) (string, error) {
	file, err := os.Open(dictPath)
	if err != nil {
		return text, err
	}
	defer file.Close()

	// replacerArgs 格式为 [old1, new1, old2, new2, ...]
	var replacerArgs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 忽略注释和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 使用 Tab 分隔，并且只分隔成两部分，防止目标词中也包含Tab
		// parts := strings.SplitN(line, "\t", 2)
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

	// 如果没有有效的替换规则，直接返回
	if len(replacerArgs) == 0 {
		return text, nil
	}

	// 创建并使用 replacer
	replacer := strings.NewReplacer(replacerArgs...)
	return replacer.Replace(text), nil
}

var reParagraphSeparator = regexp.MustCompile(`\n{2,}`)

func formatTextStream(reader io.Reader, options []string) string {
	opts := make(map[string]bool)
	for _, opt := range options {
		opts[opt] = true
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		log.Println("读取流失败:", err)
		return ""
	}
	text := string(content)

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	if opts["使用自定义字典"] {
		dictPath := filepath.Join("data", "custom_dict.txt")
		var replaceErr error
		text, replaceErr = applyCustomDictionary(text, dictPath)
		if replaceErr != nil {
			// 只是记录错误，不中断流程，保留第一步转换的结果
			log.Printf("应用自定义字典时出错: %v", replaceErr)
		}
	}

	if opts["转换为简体字"] {
		dicter := sat.DefaultDict()
		text = dicter.Read(text)

	}

	paragraphs := reParagraphSeparator.Split(text, -1)
	var processedParagraphs []string

	for _, p := range paragraphs {
		currentParagraph := p

		if opts["合并换行"] {
			fields := strings.Fields(currentParagraph)
			currentParagraph = strings.Join(fields, " ")
		} else if opts["删除非段落换行"] {
			currentParagraph = strings.ReplaceAll(currentParagraph, "\n", "")
		}

		trimmedParagraph := strings.TrimSpace(currentParagraph)
		if trimmedParagraph == "" {
			continue
		}

		if opts["段首缩进"] {
			processedParagraphs = append(processedParagraphs, "　　"+trimmedParagraph)
		} else {
			processedParagraphs = append(processedParagraphs, trimmedParagraph)
		}
	}

	return strings.Join(processedParagraphs, "\n\n")
}
