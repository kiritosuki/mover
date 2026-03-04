package config

import (
	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var VP *viper.Viper

// InitViper 初始化 viper 并读取和加载配置文件
func InitViper() {
	// viper 定位配置文件
	Logger.Logger.Info("viper 初始化配置文件...")
	VP = viper.New()
	VP.SetConfigName("config-dev")
	VP.SetConfigType("yml")
	VP.AddConfigPath(".")

	// viper 读取并加载配置文件
	err := VP.ReadInConfig()
	if err != nil {
		Logger.Logger.Error("viper 读取并加载配置文件失败", zap.Error(err))
		panic(err)
	}
	Logger.Logger.Info("viper 读取并加载配置文件成功")
}
