package global

import (
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// 定义全局变量
var (
	DB     *gorm.DB
	Config *viper.Viper
)
