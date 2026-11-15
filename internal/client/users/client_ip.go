package users

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

// signClientIP связывает IP проверенного соединения и время внутренней подписью.
func (c *Client) signClientIP(request *http.Request) {
	ip := middleware.ClientIP(request.Context())
	if ip == "" || c.loginContextSecret == "" {
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(c.loginContextSecret))
	_, _ = mac.Write([]byte("login-client-ip-v1\n" + timestamp + "\n" + ip))
	request.Header.Set("X-Login-Client-IP", ip)
	request.Header.Set("X-Login-Client-Time", timestamp)
	request.Header.Set("X-Login-Client-Signature", hex.EncodeToString(mac.Sum(nil)))
}
