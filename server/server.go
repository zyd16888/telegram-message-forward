package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/zyd16888/telegram-message-forward/global"
	"github.com/zyd16888/telegram-message-forward/models"
	"github.com/zyd16888/telegram-message-forward/plugin"

)

var pluginManager *plugin.PluginManager

func InitServer(pm *plugin.PluginManager) {
	pluginManager = pm
	r := gin.Default()
	r.GET("/plugins", getPlugins)
	r.POST("/plugins", updatePlugin)
	r.GET("/chats", getChats)
	r.POST("/chats", createChat)
	r.PUT("/chats/:id", updateChat)
	r.DELETE("/chats/:id", deleteChat)
	r.GET("/chat_plugins/:chatId", getChatPlugins)
	r.POST("/chat_plugins", associatePluginToChat)
	r.DELETE("/chat_plugins/:chatId/:pluginName", disassociatePluginFromChat)
	r.Run(":8080")
	r.POST("/reload-plugins/:chatID", reloadPluginsForChat)
}

func getChats(c *gin.Context) {
	var chats []models.ChatConfig
	if err := global.DB.Find(&chats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, chats)
}

func createChat(c *gin.Context) {
	var chat models.ChatConfig
	if err := c.ShouldBindJSON(&chat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := global.DB.Create(&chat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func updateChat(c *gin.Context) {
	id := c.Param("id")
	var chat models.ChatConfig
	if err := global.DB.First(&chat, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
		return
	}

	if err := c.ShouldBindJSON(&chat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := global.DB.Model(&chat).Updates(chat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func deleteChat(c *gin.Context) {
	id := c.Param("id")
	var chat models.ChatConfig
	if err := global.DB.First(&chat, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
		return
	}

	if err := global.DB.Delete(&chat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func getChatPlugins(c *gin.Context) {
	chatId := c.Param("chatId")
	var associations []models.ChatPluginAssociation
	if err := global.DB.Where("chat_config_id = ?", chatId).Find(&associations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, associations)
}

func associatePluginToChat(c *gin.Context) {
	var association models.ChatPluginAssociation
	if err := c.ShouldBindJSON(&association); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := global.DB.Create(&association).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, association)
}

func disassociatePluginFromChat(c *gin.Context) {
	chatId := c.Param("chatId")
	pluginName := c.Param("pluginName")

	// 加入验证逻辑
	if chatId == "" || pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID or plugin name is empty"})
		return
	}

	if err := global.DB.Where("chat_config_id = ? AND plugin_name = ?", chatId, pluginName).Delete(&models.ChatPluginAssociation{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func getPlugins(c *gin.Context) {
	var pluginConfigs []models.PluginConfig
	if err := global.DB.Find(&pluginConfigs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pluginConfigs)
}

func updatePlugin(c *gin.Context) {
	var pluginConfig models.PluginConfig
	if err := c.ShouldBindJSON(&pluginConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := global.DB.Model(&models.PluginConfig{}).Where("name = ?", pluginConfig.Name).Updates(pluginConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 重新加载插件配置
	pluginFactory := &plugin.DefaultPluginFactory{}
	pluginManager = plugin.NewPluginManager(pluginFactory)
	if err := pluginManager.LoadPluginsFromDB(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func reloadPluginsForChat(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chatID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chat ID"})
		return
	}

	if err := pluginManager.LoadPluginsForChat(chatID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugins reloaded successfully"})
}