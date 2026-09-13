package parser_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"atlasdesk/internal/platform/parser"
)

func TestTextParser(t *testing.T) {
	ctx := context.Background()
	p := &parser.TextParser{}

	t.Run("成功按自然段解析", func(t *testing.T) {
		text := "第一段内容，关于 AtlasDesk 知识库。\n\n第二段内容，关于工单流转流程。\n还有紧接的一行。"
		r := bytes.NewReader([]byte(text))

		blocks, err := p.Parse(ctx, r, int64(len(text)))
		if err != nil {
			t.Fatalf("预期解析成功，实际失败: %v", err)
		}
		if len(blocks) != 2 {
			t.Fatalf("预期提取 2 个自然段，实际得到: %d", len(blocks))
		}
		if !strings.Contains(blocks[0].Content, "第一段内容") {
			t.Errorf("第一段内容不符合预期: %s", blocks[0].Content)
		}
		if !strings.Contains(blocks[1].Content, "第二段内容") {
			t.Errorf("第二段内容不符合预期: %s", blocks[1].Content)
		}
	})

	t.Run("空文件校验拦截", func(t *testing.T) {
		r := bytes.NewReader([]byte(""))
		_, err := p.Parse(ctx, r, 0)
		if !errors.Is(err, parser.ErrEmptyFile) {
			t.Fatalf("空文件预期返回 ErrEmptyFile，实际: %v", err)
		}
	})
}

func TestMarkdownParser(t *testing.T) {
	ctx := context.Background()
	p := &parser.MarkdownParser{}

	t.Run("成功提取标题层级与内容块", func(t *testing.T) {
		md := `# AtlasDesk 简介
AtlasDesk 是一款智能知识库平台。

## 系统架构
核心采用 Go 与 React 构建。

### 数据库设计
使用 PostgreSQL 与 pgvector。`

		r := bytes.NewReader([]byte(md))
		blocks, err := p.Parse(ctx, r, int64(len(md)))
		if err != nil {
			t.Fatalf("Markdown 解析失败: %v", err)
		}

		if len(blocks) != 3 {
			t.Fatalf("预期 3 个章节块，实际得到: %d", len(blocks))
		}

		if blocks[0].HeadingPath != "AtlasDesk 简介" || !strings.Contains(blocks[0].Content, "智能知识库平台") {
			t.Errorf("第 1 块不匹配: %+v", blocks[0])
		}
		if blocks[1].HeadingPath != "系统架构" || !strings.Contains(blocks[1].Content, "核心采用") {
			t.Errorf("第 2 块不匹配: %+v", blocks[1])
		}
		if blocks[2].HeadingPath != "数据库设计" || !strings.Contains(blocks[2].Content, "pgvector") {
			t.Errorf("第 3 块不匹配: %+v", blocks[2])
		}
	})
}

func TestDocxParser(t *testing.T) {
	ctx := context.Background()
	p := &parser.DocxParser{}

	t.Run("成功解析符合 OpenXML 规范的 DOCX", func(t *testing.T) {
		// 动态构造一个微型 docx 压缩包
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>关于我们</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>这是第一行正文说明。</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

		w, err := zw.Create("word/document.xml")
		if err != nil {
			t.Fatalf("创建 zip entry 失败: %v", err)
		}
		if _, err := w.Write([]byte(docXML)); err != nil {
			t.Fatalf("写入 docx xml 失败: %v", err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("关闭 zip writer 失败: %v", err)
		}

		data := buf.Bytes()
		r := bytes.NewReader(data)
		blocks, err := p.Parse(ctx, r, int64(len(data)))
		if err != nil {
			t.Fatalf("DocxParser 解析失败: %v", err)
		}

		if len(blocks) != 1 {
			t.Fatalf("预期提取 1 个内容块，实际得到: %d", len(blocks))
		}
		if blocks[0].HeadingPath != "关于我们" {
			t.Errorf("HeadingPath 提取有误: %s", blocks[0].HeadingPath)
		}
		if !strings.Contains(blocks[0].Content, "这是第一行正文说明") {
			t.Errorf("Content 提取有误: %s", blocks[0].Content)
		}
	})

	t.Run("损坏的 DOCX 返回 ErrCorruptedFile", func(t *testing.T) {
		corruptedData := []byte("not a valid zip file")
		r := bytes.NewReader(corruptedData)
		_, err := p.Parse(ctx, r, int64(len(corruptedData)))
		if !errors.Is(err, parser.ErrCorruptedFile) {
			t.Fatalf("损坏 DOCX 预期返回 ErrCorruptedFile，实际: %v", err)
		}
	})
}

func TestGetParser(t *testing.T) {
	_, err := parser.GetParser("text/markdown")
	if err != nil {
		t.Errorf("获取 markdown 解析器失败: %v", err)
	}

	_, err = parser.GetParser("application/pdf")
	if err != nil {
		t.Errorf("获取 pdf 解析器失败: %v", err)
	}

	_, err = parser.GetParser("application/unknown-binary")
	if !errors.Is(err, parser.ErrUnsupportedFormat) {
		t.Errorf("未知 MIME 预期返回 ErrUnsupportedFormat，实际: %v", err)
	}
}

func TestPDFParser(t *testing.T) {
	ctx := context.Background()
	p := &parser.PDFParser{}

	t.Run("空文件返回 ErrEmptyFile", func(t *testing.T) {
		r := bytes.NewReader([]byte(""))
		_, err := p.Parse(ctx, r, 0)
		if !errors.Is(err, parser.ErrEmptyFile) {
			t.Fatalf("空 PDF 预期返回 ErrEmptyFile，实际: %v", err)
		}
	})

	t.Run("非法或损坏的 PDF 返回 ErrCorruptedFile", func(t *testing.T) {
		invalidPDF := []byte("%PDF-1.4\ninvalid binary stream")
		r := bytes.NewReader(invalidPDF)
		_, err := p.Parse(ctx, r, int64(len(invalidPDF)))
		if !errors.Is(err, parser.ErrCorruptedFile) {
			t.Fatalf("损坏 PDF 预期返回 ErrCorruptedFile，实际: %v", err)
		}
	})
}
