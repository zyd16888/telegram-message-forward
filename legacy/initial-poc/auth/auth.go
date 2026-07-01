package auth

import (
	"errors"
	"github.com/celestix/gotgproto"
	"sync"
)

type WebAuthConvertor struct {
	phoneNumber string
	code        string
	password    string
	status      gotgproto.AuthStatus
	mu          sync.Mutex
}

func NewWebAuthConvertor() *WebAuthConvertor {
	return &WebAuthConvertor{}
}

func (w *WebAuthConvertor) AskPhoneNumber() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.phoneNumber == "" {
		return "", errors.New("phone number not provided")
	}
	return w.phoneNumber, nil
}

func (w *WebAuthConvertor) AskCode() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.code == "" {
		return "", errors.New("code not provided")
	}
	return w.code, nil
}

func (w *WebAuthConvertor) AskPassword() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.password == "" {
		return "", errors.New("password not provided")
	}
	return w.password, nil
}

func (w *WebAuthConvertor) AuthStatus(authStatus gotgproto.AuthStatus) {
	// 处理认证状态的更新
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = authStatus
}

// 设置认证信息的方法
func (w *WebAuthConvertor) SetPhoneNumber(phone string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.phoneNumber = phone
}

func (w *WebAuthConvertor) SetCode(code string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.code = code
}

func (w *WebAuthConvertor) SetPassword(password string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.password = password
}
