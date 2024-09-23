package global

import (
	"github.com/zyd16888/telegram-message-forward/config"
	"github.com/zyd16888/telegram-message-forward/database"
)

func Init() {

	Config = config.InitConfig()

	DB = database.InitDB(Config.GetString("database"))

	database.MigrateTables(DB)
}
