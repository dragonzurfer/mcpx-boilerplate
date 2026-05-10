package routes

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

const (
	desktopLoginSessionTTL      = 10 * time.Minute
	desktopLoginPollIntervalSec = 2
)

type DesktopHandler struct {
	Store  *stores.Store
	Logger *log.Logger
}

func (h *DesktopHandler) RegisterPublic(rg *gin.RouterGroup) {
	rg.POST("/desktop/login/start", h.startLogin)
	rg.POST("/desktop/login/poll", h.pollLogin)
}

func (h *DesktopHandler) RegisterAuthed(rg *gin.RouterGroup) {
	rg.POST("/desktop/login/approve", h.approveLogin)
	rg.GET("/desktop/me", h.me)
	rg.GET("/desktop/download", h.downloadInstaller)
	rg.POST("/desktop/exports/authorize", h.authorizeExport)
	rg.POST("/desktop/exports/complete", h.completeExport)
}

type desktopDeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

type desktopLoginStartRequest struct {
	desktopDeviceInfo
}

type desktopLoginPollRequest struct {
	DeviceCode string `json:"device_code" binding:"required"`
}

type desktopLoginApproveRequest struct {
	UserCode string `json:"user_code" binding:"required"`
}

type desktopExportAuthorizeRequest struct {
	desktopDeviceInfo
	VideoDurationMs int `json:"video_duration_ms"`
	VideoWidth      int `json:"video_width"`
	VideoHeight     int `json:"video_height"`
}

type desktopExportCompleteRequest struct {
	desktopDeviceInfo
	ClientExportID  string `json:"client_export_id" binding:"required"`
	VideoDurationMs int    `json:"video_duration_ms"`
	VideoWidth      int    `json:"video_width"`
	VideoHeight     int    `json:"video_height"`
}

func (h *DesktopHandler) startLogin(c *gin.Context) {
	if h.Store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	var req desktopLoginStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Device metadata is optional; allow empty payloads from older desktop builds.
		req = desktopLoginStartRequest{}
	}

	expiresAt := time.Now().UTC().Add(desktopLoginSessionTTL)
	var session *stores.DesktopLoginSessionModel
	var err error
	for range 5 {
		deviceCode, codeErr := randomHexToken(24)
		if codeErr != nil {
			err = codeErr
			break
		}
		userCode, codeErr := randomUserCode()
		if codeErr != nil {
			err = codeErr
			break
		}
		session, err = h.Store.CreateDesktopLoginSession(stores.DesktopLoginSessionCreateInput{
			DeviceCode:     deviceCode,
			UserCode:       userCode,
			DeviceID:       strings.TrimSpace(req.DeviceID),
			DeviceName:     strings.TrimSpace(req.DeviceName),
			DevicePlatform: strings.TrimSpace(req.Platform),
			AppVersion:     strings.TrimSpace(req.AppVersion),
			ExpiresAt:      expiresAt,
		})
		if err == nil {
			break
		}
	}
	if err != nil || session == nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/login/start]: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create login session"})
		return
	}

	verifyURL := desktopVerifyURL()
	completeURL := verifyURL + "?code=" + session.UserCode
	c.JSON(http.StatusOK, gin.H{
		"status":                stores.DesktopLoginSessionStatusPending,
		"device_code":           session.DeviceCode,
		"user_code":             session.UserCode,
		"verification_url":      verifyURL,
		"verification_url_full": completeURL,
		"expires_at":            session.ExpiresAt,
		"poll_interval_seconds": desktopLoginPollIntervalSec,
		"subscription_url":      desktopSubscriptionURL(),
	})
}

func (h *DesktopHandler) pollLogin(c *gin.Context) {
	if h.Store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	var req desktopLoginPollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_code required"})
		return
	}

	now := time.Now().UTC()
	_ = h.Store.TouchDesktopLoginSessionPoll(req.DeviceCode, now)

	session, err := h.Store.GetDesktopLoginSessionByDeviceCode(req.DeviceCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "login session not found"})
		return
	}

	if now.After(session.ExpiresAt) && session.Status != stores.DesktopLoginSessionStatusApproved {
		_ = h.Store.ExpireDesktopLoginSession(session.DeviceCode, now)
		c.JSON(http.StatusOK, gin.H{
			"status":     stores.DesktopLoginSessionStatusExpired,
			"expires_at": session.ExpiresAt,
		})
		return
	}

	if session.Status != stores.DesktopLoginSessionStatusApproved || session.UserID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":     stores.DesktopLoginSessionStatusPending,
			"user_code":  session.UserCode,
			"expires_at": session.ExpiresAt,
		})
		return
	}

	user, err := h.Store.GetUserByID(session.UserID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/login/poll]: load user: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	token, err := h.issueJWTForUser(user.ID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/login/poll]: issue jwt: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}

	if strings.TrimSpace(session.DeviceID) != "" {
		_, _ = h.Store.UpsertDesktopDevice(stores.DesktopDeviceUpsertInput{
			UserID:     user.ID,
			DeviceID:   session.DeviceID,
			DeviceName: session.DeviceName,
			Platform:   session.DevicePlatform,
			AppVersion: session.AppVersion,
			Now:        now,
		})
	}

	account, err := h.buildDesktopAccountPayload(user, now)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/login/poll]: build account: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": stores.DesktopLoginSessionStatusApproved,
		"token":  token,
		"user": gin.H{
			"id":     user.ID,
			"name":   user.Name,
			"email":  user.Email,
			"avatar": user.AvatarURL,
		},
		"account": account,
	})
}

func (h *DesktopHandler) approveLogin(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req desktopLoginApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_code required"})
		return
	}

	session, err := h.Store.ApproveDesktopLoginSession(stores.DesktopLoginSessionApproveInput{
		UserCode: req.UserCode,
		UserID:   user.ID,
		Now:      time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "code invalid or expired"})
			return
		}
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/login/approve]: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"user_code": session.UserCode,
		"status":    session.Status,
	})
}

func (h *DesktopHandler) me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	now := time.Now().UTC()
	account, err := h.buildDesktopAccountPayload(user, now)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/me]: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":     user.ID,
			"name":   user.Name,
			"email":  user.Email,
			"avatar": user.AvatarURL,
			"role":   user.Role,
		},
		"account": account,
	})
}

func (h *DesktopHandler) authorizeExport(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req desktopExportAuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = desktopExportAuthorizeRequest{}
	}
	now := time.Now().UTC()
	h.touchDesktopDevice(user.ID, req.desktopDeviceInfo, now)

	account, err := h.buildDesktopAccountPayload(user, now)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/exports/authorize]: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check access"})
		return
	}

	entitlementActive, _ := account["entitlement_active"].(bool)
	freeRemaining, _ := account["free_exports_remaining"].(int)

	allowed := entitlementActive || freeRemaining > 0
	reason := "FREE_QUOTA"
	if entitlementActive {
		reason = "ENTITLED"
	} else if !allowed {
		reason = "FREE_LIMIT_REACHED"
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed": allowed,
		"reason":  reason,
		"account": account,
	})
}

func (h *DesktopHandler) completeExport(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req desktopExportCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_export_id required"})
		return
	}

	now := time.Now().UTC()
	h.touchDesktopDevice(user.ID, req.desktopDeviceInfo, now)

	if _, err := h.Store.RecordDesktopExportCompletion(stores.DesktopExportCompletionInput{
		UserID:          user.ID,
		DeviceID:        strings.TrimSpace(req.DeviceID),
		ClientExportID:  req.ClientExportID,
		VideoDurationMs: req.VideoDurationMs,
		VideoWidth:      req.VideoWidth,
		VideoHeight:     req.VideoHeight,
		AppVersion:      req.AppVersion,
		Now:             now,
	}); err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/exports/complete]: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record export"})
		return
	}

	account, err := h.buildDesktopAccountPayload(user, now)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("error [desktop/exports/complete]: account: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"account": account,
	})
}

type desktopInstallerDownload struct {
	FilePath       string
	RedirectURL    string
	AttachmentName string
}

func (h *DesktopHandler) downloadInstaller(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	platform := normalizeDesktopDownloadPlatform(strings.TrimSpace(c.Query("platform")), c.Request.UserAgent())
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform must be mac or windows"})
		return
	}
	arch := normalizeDesktopDownloadArch(platform, strings.TrimSpace(c.Query("arch")), c.Request.UserAgent())

	installer, err := resolveDesktopInstallerDownload(platform, arch)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Printf("warn [desktop/download]: user=%d platform=%s arch=%s err=%v", user.ID, platform, arch, err)
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "installer not available"})
		return
	}

	if installer.RedirectURL != "" {
		c.Redirect(http.StatusTemporaryRedirect, installer.RedirectURL)
		return
	}

	info, statErr := os.Stat(installer.FilePath)
	if statErr != nil || info.IsDir() {
		if h.Logger != nil {
			h.Logger.Printf("warn [desktop/download]: file missing path=%s err=%v", installer.FilePath, statErr)
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "installer file missing"})
		return
	}

	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(installer.FilePath, installer.AttachmentName)
}

func (h *DesktopHandler) touchDesktopDevice(userID uint, device desktopDeviceInfo, now time.Time) {
	if h.Store == nil || userID == 0 || strings.TrimSpace(device.DeviceID) == "" {
		return
	}
	_, _ = h.Store.UpsertDesktopDevice(stores.DesktopDeviceUpsertInput{
		UserID:     userID,
		DeviceID:   strings.TrimSpace(device.DeviceID),
		DeviceName: strings.TrimSpace(device.DeviceName),
		Platform:   strings.TrimSpace(device.Platform),
		AppVersion: strings.TrimSpace(device.AppVersion),
		Now:        now,
	})
}

func normalizeDesktopDownloadPlatform(raw string, userAgent string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mac", "macos", "darwin", "osx":
		return "mac"
	case "windows", "win", "win32":
		return "windows"
	}

	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "windows"):
		return "windows"
	case strings.Contains(ua, "macintosh"), strings.Contains(ua, "mac os"):
		return "mac"
	default:
		return ""
	}
}

func normalizeDesktopDownloadArch(platform string, raw string, userAgent string) string {
	if platform == "windows" {
		return "x64"
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "arm64", "aarch64", "apple-silicon":
		return "arm64"
	case "x64", "amd64", "intel":
		return "x64"
	}

	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "arm64") || strings.Contains(ua, "aarch64") {
		return "arm64"
	}
	return "arm64"
}

func resolveDesktopInstallerDownload(platform string, arch string) (*desktopInstallerDownload, error) {
	switch platform {
	case "mac":
		if arch != "x64" && arch != "arm64" {
			arch = "arm64"
		}
		if url := desktopDownloadURLFor(platform, arch); url != "" {
			return &desktopInstallerDownload{RedirectURL: url}, nil
		}
		filePath, err := desktopDownloadFilePathFor(platform, arch)
		if err != nil {
			return nil, err
		}
		return &desktopInstallerDownload{
			FilePath:       filePath,
			AttachmentName: filepath.Base(filePath),
		}, nil
	case "windows":
		if url := desktopDownloadURLFor("windows", "x64"); url != "" {
			return &desktopInstallerDownload{RedirectURL: url}, nil
		}
		filePath, err := desktopDownloadFilePathFor("windows", "x64")
		if err != nil {
			return nil, err
		}
		return &desktopInstallerDownload{
			FilePath:       filePath,
			AttachmentName: filepath.Base(filePath),
		}, nil
	default:
		return nil, errors.New("unsupported platform")
	}
}

func desktopDownloadURLFor(platform string, arch string) string {
	switch platform {
	case "mac":
		if arch == "x64" {
			if v := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_MAC_X64_URL")); v != "" {
				return v
			}
		}
		if arch == "arm64" {
			if v := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_MAC_ARM64_URL")); v != "" {
				return v
			}
		}
		return strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_MAC_URL"))
	case "windows":
		if v := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_WIN_X64_URL")); v != "" {
			return v
		}
		return strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_WINDOWS_URL"))
	default:
		return ""
	}
}

func desktopDownloadFilePathFor(platform string, arch string) (string, error) {
	switch platform {
	case "mac":
		if arch == "x64" {
			if path := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_MAC_X64_PATH")); path != "" {
				return path, nil
			}
			if path, err := latestDesktopArtifactPath(filepath.Join(desktopDownloadArtifactsDir(), "OneSub-*-mac-x64.dmg")); err == nil {
				return path, nil
			}
			return "", errors.New("mac x64 dmg not found")
		}
		if path := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_MAC_ARM64_PATH")); path != "" {
			return path, nil
		}
		if path := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_MAC_PATH")); path != "" {
			return path, nil
		}
		if path, err := latestDesktopArtifactPath(filepath.Join(desktopDownloadArtifactsDir(), "OneSub-*-mac-arm64.dmg")); err == nil {
			return path, nil
		}
		if path, err := latestDesktopArtifactPath(filepath.Join(desktopDownloadArtifactsDir(), "OneSub-*-mac-x64.dmg")); err == nil {
			return path, nil
		}
		return "", errors.New("mac dmg not found")
	case "windows":
		if path := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_WIN_X64_PATH")); path != "" {
			return path, nil
		}
		if path := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_WINDOWS_PATH")); path != "" {
			return path, nil
		}
		if path, err := latestDesktopArtifactPath(filepath.Join(desktopDownloadArtifactsDir(), "OneSub-*-win-x64.exe")); err == nil {
			return path, nil
		}
		return "", errors.New("windows installer not found")
	default:
		return "", errors.New("unsupported platform")
	}
}

func desktopDownloadArtifactsDir() string {
	if configured := strings.TrimSpace(os.Getenv("DESKTOP_DOWNLOAD_DIR")); configured != "" {
		return configured
	}
	return filepath.Clean(filepath.Join("..", "onesub-desktop", "apps", "desktop", "release"))
}

func latestDesktopArtifactPath(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}

	var bestPath string
	var bestTime time.Time
	for _, candidate := range matches {
		info, statErr := os.Stat(candidate)
		if statErr != nil || info.IsDir() {
			continue
		}
		if bestPath == "" || info.ModTime().After(bestTime) {
			bestPath = candidate
			bestTime = info.ModTime()
		}
	}
	if bestPath == "" {
		return "", os.ErrNotExist
	}
	return bestPath, nil
}

func (h *DesktopHandler) buildDesktopAccountPayload(user *stores.UserModel, now time.Time) (gin.H, error) {
	if h.Store == nil || user == nil || user.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	activeEntitlement, activeErr := h.Store.GetActiveEntitlement(stores.EntitlementLookupInput{
		UserID: user.ID,
		Now:    now,
	})
	entitlementActive := activeErr == nil && activeEntitlement != nil && activeEntitlement.Status == stores.EntitlementStatusActive

	entitlement := activeEntitlement
	if entitlement == nil {
		entitlement, _ = h.Store.GetLatestEntitlement(user.ID)
	}

	completedCount, err := h.Store.CountCompletedDesktopExports(user.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	limit := desktopFreeExportLimit()
	used := int(completedCount)
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	return gin.H{
		"entitlement_active":     entitlementActive,
		"entitlement":            buildDesktopEntitlementPayload(entitlement),
		"free_exports_limit":     limit,
		"free_exports_used":      used,
		"free_exports_remaining": remaining,
		"has_export_access":      entitlementActive || remaining > 0,
		"subscription_url":       desktopSubscriptionURL(),
	}, nil
}

func buildDesktopEntitlementPayload(entitlement *stores.EntitlementModel) gin.H {
	if entitlement == nil {
		return nil
	}
	return gin.H{
		"plan_code": entitlement.PlanCode,
		"status":    entitlement.Status,
		"start_at":  entitlement.StartAt,
		"end_at":    entitlement.EndAt,
	}
}

func (h *DesktopHandler) issueJWTForUser(userID uint) (string, error) {
	if h.Store == nil || userID == 0 {
		return "", errors.New("user missing")
	}

	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "", errors.New("jwt secret missing")
	}
	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		issuer = "mcpx"
	}

	identity, err := h.Store.GetPreferredIdentityForUser(userID)
	if err != nil {
		return "", err
	}
	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"sub":      identity.ProviderUserID,
		"userId":   user.ID,
		"email":    user.Email,
		"exp":      time.Now().Add(365 * 24 * time.Hour).Unix(),
		"iss":      issuer,
		"provider": identity.Provider,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func desktopFreeExportLimit() int {
	raw := strings.TrimSpace(os.Getenv("DESKTOP_FREE_EXPORT_LIMIT"))
	if raw == "" {
		return 10
	}
	var limit int
	_, err := fmt.Sscanf(raw, "%d", &limit)
	if err != nil || limit < 0 {
		return 10
	}
	return limit
}

func desktopVerifyURL() string {
	if explicit := strings.TrimSpace(os.Getenv("DESKTOP_DEVICE_VERIFY_URL")); explicit != "" {
		return strings.TrimRight(explicit, "/")
	}
	return strings.TrimRight(desktopSiteBaseURL(), "/") + "/desktop/link"
}

func desktopSubscriptionURL() string {
	if explicit := strings.TrimSpace(os.Getenv("DESKTOP_SUBSCRIPTION_URL")); explicit != "" {
		return strings.TrimRight(explicit, "/")
	}
	return strings.TrimRight(desktopSiteBaseURL(), "/") + "/pricing"
}

func desktopSiteBaseURL() string {
	if siteURL := strings.TrimSpace(os.Getenv("SITE_URL")); siteURL != "" {
		return strings.TrimRight(siteURL, "/")
	}
	return "https://explore.mcpx.in"
}

func randomHexToken(bytesN int) (string, error) {
	if bytesN <= 0 {
		return "", errors.New("bytesN must be positive")
	}
	buf := make([]byte, bytesN)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomUserCode() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), "=")
	raw = strings.ToUpper(raw)
	if len(raw) < 8 {
		return "", errors.New("failed to generate code")
	}
	return raw[:4] + "-" + raw[4:8], nil
}
