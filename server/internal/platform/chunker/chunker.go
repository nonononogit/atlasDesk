package chunker

import (
	"strings"
	"unicode"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
)

// Chunker 智能文档分块器
type Chunker struct {
	targetTokens  int
	overlapTokens int
}

// NewChunker 构造分块器，若参数<=0则应用默认配置 (500 tokens, 80 overlap)
func NewChunker(opts domain.ChunkOptions) *Chunker {
	target := opts.TargetTokens
	if target <= 0 {
		target = 500
	}
	overlap := opts.OverlapTokens
	if overlap <= 0 {
		overlap = 80
	}
	if overlap >= target {
		overlap = target / 4
	}
	return &Chunker{
		targetTokens:  target,
		overlapTokens: overlap,
	}
}

// EstimateTokens 快速且稳健地估算文本的 Token 数量
// 经验规则：中文每个汉字约 1~1.2 Token；英文单词按空格/词边界切分，1 词约 1.3 Token
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	tokens := 0
	inWord := false

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			// 中文字符
			tokens += 1
			inWord = false
		} else if unicode.IsSpace(r) || unicode.IsPunct(r) {
			// 分隔符与标点
			inWord = false
		} else {
			// 英文字符或数字
			if !inWord {
				tokens += 1
				inWord = true
			}
		}
	}

	if tokens == 0 && len(text) > 0 {
		tokens = 1
	}

	return tokens
}

// SplitBlocksToChunks 按照标题、段落、断句、Token 优先级将解析块切分为最终存储 Chunk
func (c *Chunker) SplitBlocksToChunks(
	blocks []domain.ParsedBlock,
	orgID, kbID, docID, versionID uuid.UUID,
	docName string,
	sourceVersion int,
) []domain.DocumentChunk {
	var chunks []domain.DocumentChunk
	chunkIndex := 0

	for _, block := range blocks {
		text := strings.TrimSpace(block.Content)
		if text == "" {
			continue
		}

		// 1. 将块拆解为基础语义单元（自然段落或断句）
		units := c.splitIntoUnits(text)

		var currentBuffer []string
		currentTokens := 0

		flushChunk := func() {
			if len(currentBuffer) == 0 {
				return
			}
			chunkContent := strings.Join(currentBuffer, "\n")
			tokenCount := EstimateTokens(chunkContent)

			chunks = append(chunks, domain.DocumentChunk{
				OrganizationID:    orgID,
				KnowledgeBaseID:   kbID,
				DocumentID:        docID,
				DocumentVersionID: versionID,
				ChunkIndex:        chunkIndex,
				Content:           chunkContent,
				TokenCount:        tokenCount,
				Metadata: domain.ChunkMetadata{
					DocumentName:  docName,
					HeadingPath:   block.HeadingPath,
					PageNumber:    block.PageNumber,
					SourceVersion: sourceVersion,
				},
			})
			chunkIndex++

			// 2. 构造重叠区间 (Overlap)
			// 保留当前缓冲区尾部约为 overlapTokens 的单元
			overlapBuffer := make([]string, 0)
			overlapCount := 0
			for i := len(currentBuffer) - 1; i >= 0; i-- {
				uTokens := EstimateTokens(currentBuffer[i])
				if overlapCount+uTokens <= c.overlapTokens || len(overlapBuffer) == 0 {
					overlapBuffer = append([]string{currentBuffer[i]}, overlapBuffer...)
					overlapCount += uTokens
				} else {
					break
				}
			}
			currentBuffer = overlapBuffer
			currentTokens = overlapCount
		}

		for _, unit := range units {
			uTokens := EstimateTokens(unit)
			if currentTokens+uTokens > c.targetTokens && len(currentBuffer) > 0 {
				flushChunk()
			}
			currentBuffer = append(currentBuffer, unit)
			currentTokens += uTokens
		}

		if len(currentBuffer) > 0 {
			flushChunk()
		}
	}

	return chunks
}

// splitIntoUnits 按段落与句子递归拆分，保证单元不超出 targetTokens
func (c *Chunker) splitIntoUnits(text string) []string {
	// 按空行或换行切分自然段
	paragraphs := strings.Split(text, "\n")
	var units []string

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// 若段落大小适中，直接作为单元
		if EstimateTokens(p) <= c.targetTokens {
			units = append(units, p)
			continue
		}

		// 若单段过大，按句子标点切分 (。！？!?)
		sentences := splitSentences(p)
		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			if EstimateTokens(s) <= c.targetTokens {
				units = append(units, s)
			} else {
				// 极长句子按硬字符截断
				runes := []rune(s)
				maxRuneStep := c.targetTokens
				for i := 0; i < len(runes); i += maxRuneStep {
					end := i + maxRuneStep
					if end > len(runes) {
						end = len(runes)
					}
					units = append(units, string(runes[i:end]))
				}
			}
		}
	}

	return units
}

// splitSentences 按常见中英文句尾标点断句
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for _, r := range text {
		current.WriteRune(r)
		if r == '。' || r == '！' || r == '？' || r == '!' || r == '?' || r == ';' || r == '；' {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}

	return sentences
}
