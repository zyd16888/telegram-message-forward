package config

import (
	"fmt"
	"github.com/spf13/viper"
)

func InitConfig() *viper.Viper {

	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath("..")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("error reading config file: %s", err))
	}
	return v
}
