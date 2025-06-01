package model

import (
	"gorm.io/gorm"
)

// Folder 目录
type Folder struct {
	// 表字段
	gorm.Model
	Name     string `gorm:"uniqueIndex:idx_only_one_name"`
	ParentID *uint  `gorm:"index:parent_id;uniqueIndex:idx_only_one_name"`
	OwnerID  uint   `gorm:"index:owner_id"`

	// 数据库忽略字段
	Position      string `gorm:"-"`
	WebdavDstName string `gorm:"-"`
}
