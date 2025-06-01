package model

import (
	"gorm.io/gorm"
)

// File 文件
type File struct {
	// 表字段
	gorm.Model
	Name            string `gorm:"uniqueIndex:idx_only_one"`
	SourceName      string `gorm:"type:text"`
	UserID          uint   `gorm:"index:user_id;uniqueIndex:idx_only_one"`
	Size            uint64
	PicInfo         string
	FolderID        uint `gorm:"index:folder_id;uniqueIndex:idx_only_one"`
	PolicyID        uint
	UploadSessionID *string `gorm:"index:session_id;uniqueIndex:session_only_one"`
	Metadata        string  `gorm:"type:text"`

	// 关联模型
	Policy Policy

	// 数据库忽略字段
	Position           string            `gorm:"-"`
	MetadataSerialized map[string]string `gorm:"-"`
}

// Thumb related metadata
const (
	ThumbStatusNotExist     = ""
	ThumbStatusExist        = "exist"
	ThumbStatusNotAvailable = "not_available"

	ThumbStatusMetadataKey  = "thumb_status"
	ThumbSidecarMetadataKey = "thumb_sidecar"

	ChecksumMetadataKey = "webdav_checksum"
)
