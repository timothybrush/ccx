package copilot

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	corecopilot "github.com/BenedictKing/ccx/internal/copilot"
	"github.com/gin-gonic/gin"
)

var oauthRequestTimeout = 10 * time.Second
var newOAuthClient = corecopilot.NewOAuthClient

type tokenRequest struct {
	DeviceCode string `json:"deviceCode"`
}

type verifyRequest struct {
	AccessToken string `json:"accessToken"`
}

// RequestDeviceCode 发起 GitHub Copilot OAuth Device Flow。
func RequestDeviceCode() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
		defer cancel()

		log.Printf("[Copilot-OAuth] 请求 GitHub device code: ip=%s timeout=%s", c.ClientIP(), oauthRequestTimeout)
		client := newOAuthClient(nil)
		deviceCode, err := client.RequestDeviceCode(ctx)
		if err != nil {
			log.Printf("[Copilot-OAuth] GitHub device code 请求失败: ip=%s error=%v", c.ClientIP(), err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		log.Printf("[Copilot-OAuth] GitHub device code 请求成功: ip=%s expiresIn=%d interval=%d", c.ClientIP(), deviceCode.ExpiresIn, deviceCode.Interval)
		c.JSON(http.StatusOK, gin.H{
			"deviceCode":      deviceCode.DeviceCode,
			"userCode":        deviceCode.UserCode,
			"verificationUri": deviceCode.VerificationURI,
			"expiresIn":       deviceCode.ExpiresIn,
			"interval":        deviceCode.Interval,
		})
	}
}

// PollAccessToken 轮询 GitHub OAuth access token。
func PollAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req tokenRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DeviceCode) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "deviceCode is required"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
		defer cancel()

		client := newOAuthClient(nil)
		token, err := client.PollAccessToken(ctx, req.DeviceCode)
		if err != nil {
			log.Printf("[Copilot-OAuth] GitHub access token 轮询失败: ip=%s error=%v", c.ClientIP(), err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		if token.Error != "" {
			c.JSON(http.StatusOK, gin.H{
				"error":            token.Error,
				"errorDescription": token.ErrorDescription,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"accessToken": token.AccessToken,
			"tokenType":   token.TokenType,
			"scope":       token.Scope,
		})
	}
}

// VerifyToken 验证 GitHub OAuth token 并返回用户信息。
func VerifyToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req verifyRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AccessToken) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "accessToken is required"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
		defer cancel()

		client := newOAuthClient(nil)
		user, err := client.VerifyUser(ctx, req.AccessToken)
		if err != nil {
			log.Printf("[Copilot-OAuth] GitHub token 验证失败: ip=%s error=%v", c.ClientIP(), err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"login":     user.Login,
			"id":        user.ID,
			"avatarUrl": user.AvatarURL,
			"htmlUrl":   user.HTMLURL,
		})
	}
}
