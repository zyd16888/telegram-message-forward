package server

import (
	"net/http"

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
	r.Run(":8080")
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
	if err := pluginManager.LoadPluginsFromDB(global.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}