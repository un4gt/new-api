package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func SetWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", common.EmbedFolder(buildFS, "web/dist")))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") ||
			strings.HasPrefix(c.Request.RequestURI, "/v1beta") ||
			strings.HasPrefix(c.Request.RequestURI, "/dashboard") ||
			strings.HasPrefix(c.Request.RequestURI, "/embeddings") ||
			strings.HasPrefix(c.Request.RequestURI, "/rerank") ||
			strings.HasPrefix(c.Request.RequestURI, "/api") ||
			strings.HasPrefix(c.Request.RequestURI, "/assets") ||
			strings.HasPrefix(c.Request.RequestURI, "/pg") ||
			strings.HasPrefix(c.Request.RequestURI, "/suno") ||
			strings.HasPrefix(c.Request.RequestURI, "/kling") ||
			strings.HasPrefix(c.Request.RequestURI, "/jimeng") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})
}
