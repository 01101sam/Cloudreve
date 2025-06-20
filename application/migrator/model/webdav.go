package model

import (
	"gorm.io/gorm"
)

// Webdav 应用账户
type Webdav struct {
	gorm.Model
	Name     string // 应用名称
	Password string `gorm:"uniqueIndex:password_only_on"` // 应用密码
	UserID   uint   `gorm:"uniqueIndex:password_only_on"` // 用户ID
	Root     string `gorm:"type:text"`                    // 根目录
	Readonly bool   `gorm:"type:bool"`                    // 是否只读
	UseProxy bool   `gorm:"type:bool"`                    // 是否进行反代
}
