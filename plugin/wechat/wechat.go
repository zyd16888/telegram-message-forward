package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/celestix/gotgproto/types"
)

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type WeChatPlugin struct {
	Crpid       string `json:"corpid"`
	Corpsecret  string `json:"corpsecret"`
	Agentid     string `json:"agentid"`
	AccessToken string `json:"access_token"`
	ExpireTime  time.Time
}

type SendMessageRequest struct {
	ToUser                 string `json:"touser"`
	ToParty                string `json:"toparty"`
	ToTag                  string `json:"totag"`
	MsgType                string `json:"msgtype"`
	AgentID                string `json:"agentid"`
	Text                   *Text  `json:"text"`
	Safe                   int    `json:"safe"`
	EnableIDTrans          int    `json:"enable_id_trans"`
	EnableDuplicateCheck   int    `json:"enable_duplicate_check"`
	DuplicateCheckInterval int    `json:"duplicate_check_interval"`
}
type Text struct {
	Content string `json:"content"`
}

func NewWeChatPlugin(configMap map[string]interface{}) *WeChatPlugin {
	corpid := configMap["corpid"].(string)
	corpsecret := configMap["corpsecret"].(string)
	agentid := configMap["agentid"].(string)

	return &WeChatPlugin{
		Crpid:       corpid,
		Corpsecret:  corpsecret,
		Agentid:     agentid,
		AccessToken: "",
		ExpireTime:  time.Time{},
	}
}

func (w *WeChatPlugin) getAccessToken() error {
	// 如果 access_token 未过期，直接返回
	if time.Now().Before(w.ExpireTime) {
		return nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", w.Crpid, w.Corpsecret)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get access token, status code: %d", resp.StatusCode)
	}

	var tokenResponse AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return err
	}

	w.AccessToken = tokenResponse.AccessToken
	w.ExpireTime = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second) // 设置过期时间
	return nil
}

func (w *WeChatPlugin) Handle(message *types.Message) error {
	if err := w.getAccessToken(); err != nil {
		return err
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", w.AccessToken)

	sendMessageReq := SendMessageRequest{
		AgentID: w.Agentid,
		ToUser:  "@all",
		MsgType: "text", // 或根据 message 的类型设置
		Text: &Text{
			Content: message.Text,
		},
	}

	jsonData, err := json.Marshal(sendMessageReq)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message, status code: %d", resp.StatusCode)
	} else {
		fmt.Println("send wechat message success")
	}

	return nil
}
