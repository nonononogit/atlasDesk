package parser

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"atlasdesk/internal/domain"
	"github.com/ledongthuc/pdf"
)

// 规范定义的标准错误码常量 (符合规格 5.2 节)
var (
	ErrUnsupportedFormat = errors.New("UNSUPPORTED_FORMAT")
	ErrEmptyFile         = errors.New("EMPTY_FILE")
	ErrOCRRequired       = errors.New("OCR_REQUIRED") // 扫描件/纯图片 PDF 无文本层
	ErrCorruptedFile     = errors.New("CORRUPTED_FILE")
)

// Parser 文档解析器通用接口
type Parser interface {
	Parse(ctx context.Context, r io.ReaderAt, size int64) ([]domain.ParsedBlock, error)
}

// GetParser 根据 MIME 类型获取对应的解析器实现
func GetParser(mimeType string) (Parser, error) {
	switch mimeType {
	case "text/markdown":
		return &MarkdownParser{}, nil
	case "text/plain":
		return &TextParser{}, nil
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return &DocxParser{}, nil
	case "application/pdf":
		return &PDFParser{}, nil
	default:
		return nil, fmt.Errorf("%w: 不支持的 MIME 类型 %s", ErrUnsupportedFormat, mimeType)
	}
}

// ----------------------------------------------------------------------------
// 1. TextParser: 纯文本解析器
// ----------------------------------------------------------------------------

type TextParser struct{}

func (p *TextParser) Parse(ctx context.Context, r io.ReaderAt, size int64) ([]domain.ParsedBlock, error) {
	if size == 0 {
		return nil, ErrEmptyFile
	}

	sr := io.NewSectionReader(r, 0, size)
	scanner := bufio.NewScanner(sr)

	var blocks []domain.ParsedBlock
	var currentPara strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if currentPara.Len() > 0 {
				blocks = append(blocks, domain.ParsedBlock{
					Content: currentPara.String(),
				})
				currentPara.Reset()
			}
			continue
		}

		if currentPara.Len() > 0 {
			currentPara.WriteString("\n")
		}
		currentPara.WriteString(line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: 扫描纯文本出错: %v", ErrCorruptedFile, err)
	}

	if currentPara.Len() > 0 {
		blocks = append(blocks, domain.ParsedBlock{
			Content: currentPara.String(),
		})
	}

	if len(blocks) == 0 {
		return nil, ErrEmptyFile
	}

	return blocks, nil
}

// ----------------------------------------------------------------------------
// 2. MarkdownParser: Markdown 语法结构解析器
// ----------------------------------------------------------------------------

type MarkdownParser struct{}

func (p *MarkdownParser) Parse(ctx context.Context, r io.ReaderAt, size int64) ([]domain.ParsedBlock, error) {
	if size == 0 {
		return nil, ErrEmptyFile
	}

	sr := io.NewSectionReader(r, 0, size)
	scanner := bufio.NewScanner(sr)

	var blocks []domain.ParsedBlock
	var currentHeading string
	var currentContent strings.Builder

	flushBlock := func() {
		content := strings.TrimSpace(currentContent.String())
		if content != "" {
			blocks = append(blocks, domain.ParsedBlock{
				HeadingPath: currentHeading,
				Content:     content,
			})
		}
		currentContent.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// 识别 Markdown 标题 (# 一级, ## 二级, ### 三级 等)
		if strings.HasPrefix(trimmed, "#") {
			hashes := 0
			for _, ch := range trimmed {
				if ch == '#' {
					hashes++
				} else {
					break
				}
			}

			// 检查是否有空格分隔，如 "# 标题"
			if hashes > 0 && hashes <= 6 && len(trimmed) > hashes && trimmed[hashes] == ' ' {
				flushBlock()
				title := strings.TrimSpace(trimmed[hashes:])
				currentHeading = title
				continue
			}
		}

		if currentContent.Len() > 0 {
			currentContent.WriteString("\n")
		}
		currentContent.WriteString(line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: 扫描 Markdown 文件失败: %v", ErrCorruptedFile, err)
	}

	flushBlock()

	if len(blocks) == 0 {
		return nil, ErrEmptyFile
	}

	return blocks, nil
}

// ----------------------------------------------------------------------------
// 3. DocxParser: 纯 Go 原生解压与 XML 段落解析器
// ----------------------------------------------------------------------------

type DocxParser struct{}

// XML 结构映射
type docxBody struct {
	XMLName    xml.Name        `xml:"body"`
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Properties *docxPPr   `xml:"pPr"`
	Runs       []docxRun  `xml:"r"`
}

type docxPPr struct {
	Style *docxStyle `xml:"pStyle"`
}

type docxStyle struct {
	Val string `xml:"val,attr"`
}

type docxRun struct {
	Text string `xml:"t"`
}

type docxDocumentRoot struct {
	XMLName xml.Name `xml:"document"`
	Body    docxBody `xml:"body"`
}

func (p *DocxParser) Parse(ctx context.Context, r io.ReaderAt, size int64) ([]domain.ParsedBlock, error) {
	if size == 0 {
		return nil, ErrEmptyFile
	}

	zipReader, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法作为 ZIP 结构解压 DOCX 文件: %v", ErrCorruptedFile, err)
	}

	var docXMLFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			docXMLFile = f
			break
		}
	}

	if docXMLFile == nil {
		return nil, fmt.Errorf("%w: DOCX 缺少关键的 word/document.xml 结构", ErrCorruptedFile)
	}

	rc, err := docXMLFile.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: 读取 word/document.xml 失败: %v", ErrCorruptedFile, err)
	}
	defer rc.Close()

	xmlData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取 XML 失败: %v", ErrCorruptedFile, err)
	}

	var root docxDocumentRoot
	if err := xml.Unmarshal(xmlData, &root); err != nil {
		return nil, fmt.Errorf("%w: 解析 DOCX XML 失败: %v", ErrCorruptedFile, err)
	}

	var blocks []domain.ParsedBlock
	var currentHeading string
	var currentSection strings.Builder

	flushSection := func() {
		text := strings.TrimSpace(currentSection.String())
		if text != "" {
			blocks = append(blocks, domain.ParsedBlock{
				HeadingPath: currentHeading,
				Content:     text,
			})
		}
		currentSection.Reset()
	}

	for _, para := range root.Body.Paragraphs {
		// 收集段落内所有文本
		var paraText strings.Builder
		for _, run := range para.Runs {
			paraText.WriteString(run.Text)
		}
		text := strings.TrimSpace(paraText.String())
		if text == "" {
			continue
		}

		// 检查是否为标题样式 (如 Heading1, Heading2, 标题 1 等)
		isHeading := false
		if para.Properties != nil && para.Properties.Style != nil {
			styleVal := strings.ToLower(para.Properties.Style.Val)
			if strings.Contains(styleVal, "heading") || strings.Contains(styleVal, "title") || strings.Contains(styleVal, "header") {
				isHeading = true
			}
		}

		if isHeading {
			flushSection()
			currentHeading = text
		} else {
			if currentSection.Len() > 0 {
				currentSection.WriteString("\n")
			}
			currentSection.WriteString(text)
		}
	}

	flushSection()

	if len(blocks) == 0 {
		return nil, ErrEmptyFile
	}

	return blocks, nil
}

// ----------------------------------------------------------------------------
// 4. PDFParser: 原生 PDF 文本层提取与无文本扫描件 OCR_REQUIRED 判定
// ----------------------------------------------------------------------------

type PDFParser struct{}

func (p *PDFParser) Parse(ctx context.Context, r io.ReaderAt, size int64) ([]domain.ParsedBlock, error) {
	if size == 0 {
		return nil, ErrEmptyFile
	}

	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: 打开 PDF 结构失败: %v", ErrCorruptedFile, err)
	}

	numPages := reader.NumPage()
	if numPages <= 0 {
		return nil, ErrEmptyFile
	}

	var blocks []domain.ParsedBlock
	totalTextLength := 0

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			// 若某页解析报错，记录并继续
			continue
		}

		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			totalTextLength += len(trimmed)
			blocks = append(blocks, domain.ParsedBlock{
				PageNumber: i,
				Content:    trimmed,
			})
		}
	}

	// 关键判定：若所有页面提取出的字符数均为 0，属于无文字层的扫描件或纯图片 PDF
	// 规范要求：抛出 OCR_REQUIRED，引导用户或提示 OCR 需求
	if totalTextLength == 0 {
		return nil, ErrOCRRequired
	}

	return blocks, nil
}
