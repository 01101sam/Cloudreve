package model

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/cloudreve/Cloudreve/v4/application/migrator/conf"
	"github.com/cloudreve/Cloudreve/v4/pkg/util"
)

// DB 数据库链接单例
var DB *gorm.DB

// Init 初始化 MySQL 链接
func Init() error {
	var (
		db         *gorm.DB
		err        error
		confDBType string = conf.DatabaseConfig.Type
	)

	// 兼容已有配置中的 "sqlite3" 配置项
	if confDBType == "sqlite3" {
		confDBType = "sqlite"
	}

	// Configure GORM v2 with table prefix and logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Debug mode
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: conf.DatabaseConfig.TablePrefix,
		},
	}

	switch confDBType {
	case "UNSET", "sqlite":
		// 未指定数据库或者明确指定为 sqlite 时，使用 SQLite 数据库
		db, err = gorm.Open(sqlite.Open(util.RelativePath(conf.DatabaseConfig.DBFile)), gormConfig)
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
			conf.DatabaseConfig.Host,
			conf.DatabaseConfig.User,
			conf.DatabaseConfig.Password,
			conf.DatabaseConfig.Name,
			conf.DatabaseConfig.Port)
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	case "mysql":
		var host string
		if conf.DatabaseConfig.UnixSocket {
			host = fmt.Sprintf("unix(%s)",
				conf.DatabaseConfig.Host)
		} else {
			host = fmt.Sprintf("(%s:%d)",
				conf.DatabaseConfig.Host,
				conf.DatabaseConfig.Port)
		}

		dsn := fmt.Sprintf("%s:%s@%s/%s?charset=%s&parseTime=True&loc=Local",
			conf.DatabaseConfig.User,
			conf.DatabaseConfig.Password,
			host,
			conf.DatabaseConfig.Name,
			conf.DatabaseConfig.Charset)
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)
	case "mssql":
		dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			conf.DatabaseConfig.User,
			conf.DatabaseConfig.Password,
			conf.DatabaseConfig.Host,
			conf.DatabaseConfig.Port,
			conf.DatabaseConfig.Name)
		db, err = gorm.Open(sqlserver.Open(dsn), gormConfig)
	default:
		return fmt.Errorf("unsupported database type %q", confDBType)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	//设置连接池
	sqlDB.SetMaxIdleConns(50)
	if confDBType == "sqlite" || confDBType == "UNSET" {
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(100)
	}

	//超时
	sqlDB.SetConnMaxLifetime(time.Second * 30)

	DB = db

	return nil
}
