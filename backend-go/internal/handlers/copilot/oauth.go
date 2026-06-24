package copilot

import (
	"net/http"
	"strings"

	corecopilot "github.com/BenedictKing/ccx/internal/copilot"
	"github.com/gin-gonic/gin"
)

type tokenRequest struct {
	DeviceCode string `json:"deviceCode"`
}

type verifyRequest struct {
	AccessToken string `json:"accessToken"`
}

// RequestDeviceCode 发起 GitHub Copilot OAuth Device Flow。
func RequestDeviceCode() gin.HandlerFunc {
	return func(c *gin.Context) {
		client := corecopilot.NewOAuthClient(nil)
		deviceCode, err := client.RequestDeviceCode(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

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

		client := corecopilot.NewOAuthClient(nil)
		token, err := client.PollAccessToken(c.Request.Context(), req.DeviceCode)
		if err != nil {
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

		client := corecopilot.NewOAuthClient(nil)
		user, err := client.VerifyUser(c.Request.Context(), req.AccessToken)
		if err != nil {
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
