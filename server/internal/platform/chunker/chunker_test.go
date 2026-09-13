package chunker_test

import (
	"strings"
	"testing"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/platform/chunker"
	"github.com/google/uuid"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"Hello World", 2},
		{"你好，世界！", 4},
		{"AtlasDesk 知识库平台", 6},
	}

	for _, tc := range tests {
		actual := chunker.EstimateTokens(tc.input)
		if actual < tc.expected-1 || actual > tc.expected+1 {
			t.Errorf("估算 Token 偏离: 输入 [%s], 期望约 %d, 实际 %d", tc.input, tc.expected, actual)
		}
	}
}

func TestSplitBlocksToChunks(t *testing.T) {
	c := chunker.NewChunker(domain.ChunkOptions{
		TargetTokens:  20,
		OverlapTokens: 5,
	})

	orgID := uuid.New()
	kbID := uuid.New()
	docID := uuid.New()
	versionID := uuid.New()
	docName := "测试指南.md"
	version := 1

	blocks := []domain.ParsedBlock{
		{
			HeadingPath: "第一章 引言",
			PageNumber:  1,
			Content:     "这是第一段介绍内容。欢迎使用 AtlasDesk。\n这是第二段介绍，阐述核心功能与定位。",
		},
		{
			HeadingPath: "第二章 安装部署",
			PageNumber:  2,
			Content:     "安装非常简单。只需要执行 make dev 即可拉起容器并启动前后端。\n更多详细文档请参考架构规范说明。",
		},
	}

	chunks := c.SplitBlocksToChunks(blocks, orgID, kbID, docID, versionID, docName, version)

	if len(chunks) == 0 {
		t.Fatalf("预期产生切片，实际为空")
	}

	// 校验切片元数据与索引单调递增
	for i, chunk := range chunks {
		if chunk.ChunkIndex != i {
			t.Errorf("切片索引异常: 期望 %d, 实际 %d", i, chunk.ChunkIndex)
		}
		if chunk.DocumentID != docID || chunk.DocumentVersionID != versionID {
			t.Errorf("切片关联 ID 不正确: %+v", chunk)
		}
		if chunk.Metadata.DocumentName != docName {
			t.Errorf("切片元数据文档名缺失: %s", chunk.Metadata.DocumentName)
		}
		if chunk.TokenCount <= 0 {
			t.Errorf("切片 TokenCount 必须大于0，实际: %d", chunk.TokenCount)
		}
	}

	// 验证第二章的切片包含对应 HeadingPath 与 PageNumber，并覆盖关键正文
	foundChapter2 := false
	foundMakeDev := false
	for _, chunk := range chunks {
		if chunk.Metadata.HeadingPath == "第二章 安装部署" {
			foundChapter2 = true
			if chunk.Metadata.PageNumber != 2 {
				t.Errorf("第二章页码有误: 期望 2, 实际 %d", chunk.Metadata.PageNumber)
			}
			if strings.Contains(chunk.Content, "make dev") {
				foundMakeDev = true
			}
		}
	}
	if !foundChapter2 {
		t.Errorf("未找到第二章对应的切片")
	}
	if !foundMakeDev {
		t.Errorf("第二章切片中应包含关键命令 make dev")
	}
}
