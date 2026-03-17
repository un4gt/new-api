package router

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	SetApiRouter(router)
	SetRelayRouter(router)
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	if common.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, buildFS, indexPage)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(func(c *gin.Context) {
			c.Set(middleware.RouteTagKey, "web")
			requestURI := c.Request.RequestURI
			// Do not redirect API/relay routes to the external frontend.
			if strings.HasPrefix(requestURI, "/v1") ||
				strings.HasPrefix(requestURI, "/v1beta") ||
				strings.HasPrefix(requestURI, "/dashboard") ||
				strings.HasPrefix(requestURI, "/api") ||
				strings.HasPrefix(requestURI, "/embeddings") ||
				strings.HasPrefix(requestURI, "/rerank") ||
				strings.HasPrefix(requestURI, "/pg") ||
				strings.HasPrefix(requestURI, "/mj") ||
				strings.HasPrefix(requestURI, "/suno") ||
				strings.HasPrefix(requestURI, "/kling") ||
				strings.HasPrefix(requestURI, "/jimeng") {
				controller.RelayNotFound(c)
				return
			}
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, requestURI))
		})
	}
}
