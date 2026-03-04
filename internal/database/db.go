package database

import (
	"fmt"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/config"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 数据库连接对象
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() {
	Logger.Logger.Info("初始化数据库连接...")
	// 获取数据库名称
	dbname := config.VP.GetString("database.dbname")
	// 获取数据库运行 ip
	ip := config.VP.GetString("database.host")
	// 获取数据库运行端口
	port := config.VP.GetString("database.port")
	// 获取数据库用户名
	username := config.VP.GetString("database.username")
	// 获取数据库密码
	password := config.VP.GetString("database.password")

	// DataSourceName 数据库连接字符串
	// 格式：<username>:<password>@tcp(<ip>:<port>)/<数据库名>?<参数设置>=<...>
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, ip, port, dbname)

	// 创建数据库连接对象
	var err error
	DB, err = gorm.Open(mysql.Open(dsn))
	if err != nil {
		Logger.Logger.Error("数据库连接失败！", zap.Error(err))
		panic(err)
	}
	Logger.Logger.Info("数据库连接成功")
}
