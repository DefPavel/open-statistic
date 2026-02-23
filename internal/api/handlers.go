package api

import (
	"net/http"

	"open-statistic/internal/database"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db           *database.DB
	collectFn    CollectFn
	allowedPaths []string // разрешённые директории для path (защита от traversal)
}

func New(db *database.DB) *Handler {
	return &Handler{db: db, allowedPaths: []string{"/var/log/openvpn"}}
}

func (h *Handler) SetCollectFn(fn CollectFn) {
	h.collectFn = fn
}

func (h *Handler) SetAllowedPaths(paths []string) {
	h.allowedPaths = paths
}

// GetUsers godoc
// @Summary Список пользователей
// @Tags users
// @Produce json
// @Success 200 {object} []string
// @Router /users [get]
func (h *Handler) GetUsers(c *gin.Context) {
	users, err := h.db.GetUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// GetUserTraffic godoc
// @Summary Трафик пользователя
// @Tags users
// @Param name path string true "Common Name пользователя"
// @Produce json
// @Success 200 {object} database.UserTraffic
// @Router /users/{name}/traffic [get]
func (h *Handler) GetUserTraffic(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "имя пользователя обязательно"})
		return
	}
	traffic, err := h.db.GetUserTraffic(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if c.Query("human") == "1" {
		c.JSON(http.StatusOK, gin.H{
			"common_name":     traffic.CommonName,
			"bytes_received": FormatBytes(traffic.BytesReceived),
			"bytes_sent":     FormatBytes(traffic.BytesSent),
			"total_bytes":    FormatBytes(traffic.TotalBytes),
		})
		return
	}
	c.JSON(http.StatusOK, traffic)
}

// GetAllTraffic godoc
// @Summary Трафик всех пользователей
// @Tags traffic
// @Produce json
// @Success 200 {array} database.UserTraffic
// @Router /traffic [get]
func (h *Handler) GetAllTraffic(c *gin.Context) {
	traffic, err := h.db.GetAllTraffic()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if c.Query("human") == "1" {
		out := make([]gin.H, 0, len(traffic))
		for _, t := range traffic {
			out = append(out, gin.H{
				"common_name":    t.CommonName,
				"bytes_received": FormatBytes(t.BytesReceived),
				"bytes_sent":     FormatBytes(t.BytesSent),
				"total_bytes":    FormatBytes(t.TotalBytes),
			})
		}
		c.JSON(http.StatusOK, gin.H{"traffic": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"traffic": traffic})
}

// GetConnected godoc
// @Summary Текущие подключения (последний снимок)
// @Tags traffic
// @Produce json
// @Success 200 {array} parser.Client
// @Router /connected [get]
func (h *Handler) GetConnected(c *gin.Context) {
	clients, err := h.db.GetLatestSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"clients": clients})
}

// GetStats godoc
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if c.Query("human") == "1" {
		c.JSON(http.StatusOK, gin.H{
			"connected_count":         stats.ConnectedCount,
			"total_users":             stats.TotalUsers,
			"session_bytes_received":  FormatBytes(stats.SessionBytesR),
			"session_bytes_sent":      FormatBytes(stats.SessionBytesS),
			"session_bytes_total":     FormatBytes(stats.SessionBytesR + stats.SessionBytesS),
			"total_bytes_received":   FormatBytes(stats.TotalBytesR),
			"total_bytes_sent":       FormatBytes(stats.TotalBytesS),
			"total_bytes_all":         FormatBytes(stats.TotalBytesR + stats.TotalBytesS),
		})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetUserTotal godoc
func (h *Handler) GetUserTotal(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "имя пользователя обязательно"})
		return
	}
	traffic, err := h.db.GetTotalTraffic(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if c.Query("human") == "1" {
		c.JSON(http.StatusOK, gin.H{
			"common_name":     traffic.CommonName,
			"bytes_received": FormatBytes(traffic.BytesReceived),
			"bytes_sent":     FormatBytes(traffic.BytesSent),
			"total_bytes":    FormatBytes(traffic.TotalBytes),
		})
		return
	}
	c.JSON(http.StatusOK, traffic)
}

// GetTotalTraffic godoc
func (h *Handler) GetTotalTraffic(c *gin.Context) {
	traffic, err := h.db.GetTotalTrafficAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if c.Query("human") == "1" {
		out := make([]gin.H, 0, len(traffic))
		for _, t := range traffic {
			out = append(out, gin.H{
				"common_name":     t.CommonName,
				"bytes_received":  FormatBytes(t.BytesReceived),
				"bytes_sent":      FormatBytes(t.BytesSent),
				"total_bytes":     FormatBytes(t.TotalBytes),
			})
		}
		c.JSON(http.StatusOK, gin.H{"traffic": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"traffic": traffic})
}

// CollectNow godoc
// @Summary Принудительно собрать статистику из status-файла
// @Tags collect
// @Param path query string true "Путь к OpenVPN status-файлу"
// @Produce json
// @Success 200 {object} map[string]int
// @Router /collect [post]
func (h *Handler) CollectNow(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "параметр path обязателен"})
		return
	}
	// Защита от path traversal
	var allowed bool
	for _, dir := range h.allowedPaths {
		if ValidatePath(path, dir) {
			allowed = true
			break
		}
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "путь не разрешён"})
		return
	}
	if h.collectFn != nil {
		if err := h.collectFn(path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CollectFn вызывается для сбора статистики (инжектируется из main)
type CollectFn func(statusPath string) error
