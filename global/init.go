package global

import (
	"telegram-message-forward/config"
	"telegram-message-forward/database"
)

func Init() {

	Config = config.InitConfig()

	DB = database.InitDB(Config.GetString("database"))

	database.MigrateTables(DB)
}
