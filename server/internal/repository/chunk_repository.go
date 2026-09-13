package repository

import (
	"context"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChunkRepository 文档切片数据仓储接口 (符合规格 5.4 节)
type ChunkRepository interface {
	// SaveChunksInTx 在数据库事务中幂等保存切片 (先清理当前版本历史切片，再批量插入)
	SaveChunksInTx(ctx context.Context, versionID uuid.UUID, chunks []domain.DocumentChunk) error
	// GetChunksByVersion 获取某个文档版本的所有切片
	GetChunksByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.DocumentChunk, error)
	// CountChunksByVersion 统计某个文档版本的切片总数
	CountChunksByVersion(ctx context.Context, versionID uuid.UUID) (int64, error)
}

// GormChunkRepository 基于 GORM 的切片仓储实现
type GormChunkRepository struct {
	db *gorm.DB
}

// NewChunkRepository 构造切片仓储
func NewChunkRepository(db *gorm.DB) *GormChunkRepository {
	return &GormChunkRepository{db: db}
}

// SaveChunksInTx 保证切片重试或重处理的绝对幂等性
func (r *GormChunkRepository) SaveChunksInTx(ctx context.Context, versionID uuid.UUID, chunks []domain.DocumentChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 清理已有历史切片，杜绝重跑导致数据重复累加
		if err := tx.Where("document_version_id = ?", versionID).Delete(&domain.DocumentChunk{}).Error; err != nil {
			return err
		}

		// 2. 批量插入新切片 (分批插入避免单条 SQL 过长)
		if len(chunks) > 0 {
			if err := tx.CreateInBatches(chunks, 100).Error; err != nil {
				return err
			}
		}

		// 3. 将 document_versions 状态置为 COMPLETED
		if err := tx.Model(&domain.DocumentVersion{}).
			Where("id = ?", versionID).
			Update("parse_status", "COMPLETED").Error; err != nil {
			return err
		}

		return nil
	})
}

// GetChunksByVersion 获取切片并按 chunk_index 正序排列
func (r *GormChunkRepository) GetChunksByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.DocumentChunk, error) {
	var chunks []domain.DocumentChunk
	err := r.db.WithContext(ctx).
		Where("document_version_id = ?", versionID).
		Order("chunk_index ASC").
		Find(&chunks).Error
	return chunks, err
}

// CountChunksByVersion 统计切片数量
func (r *GormChunkRepository) CountChunksByVersion(ctx context.Context, versionID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.DocumentChunk{}).
		Where("document_version_id = ?", versionID).
		Count(&count).Error
	return count, err
}
