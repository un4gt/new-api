package router

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func SetRelayRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	router.Use(middleware.DecompressRequestMiddleware())
	router.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	router.Use(middleware.StatsMiddleware())
	modelsRouter := router.Group("/v1/models")
	modelsRouter.Use(middleware.RouteTag("relay"))
	modelsRouter.Use(middleware.TokenAuth())
	{
		modelsRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeOpenAI)
		})

		modelsRouter.GET("/:model", func(c *gin.Context) {
			controller.RetrieveModel(c, constant.ChannelTypeOpenAI)
		})
	}
	relayV1Router := router.Group("/v1")
	relayV1Router.Use(middleware.RouteTag("relay"))
	relayV1Router.Use(middleware.SystemPerformanceCheck())
	relayV1Router.Use(middleware.TokenAuth())
	relayV1Router.Use(middleware.ModelRequestRateLimit())
	relayV1Router.Use(middleware.Distribute())
	{
		relayV1Router.POST("/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatEmbedding)
		})
		relayV1Router.POST("/rerank", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatRerank)
		})
	}

	// Minimal build: expose Gemini-compatible embedContent endpoints only for embeddings use cases.
	relayV1BetaRouter := router.Group("/v1beta")
	relayV1BetaRouter.Use(middleware.RouteTag("relay"))
	relayV1BetaRouter.Use(middleware.SystemPerformanceCheck())
	relayV1BetaRouter.Use(middleware.TokenAuth())
	relayV1BetaRouter.Use(middleware.ModelRequestRateLimit())
	relayV1BetaRouter.Use(middleware.Distribute())
	{
		relayV1BetaRouter.POST("/models/*path", func(c *gin.Context) {
			// Only allow embedding actions for the minimal build.
			if !strings.Contains(c.Request.URL.Path, ":embedContent") && !strings.Contains(c.Request.URL.Path, ":batchEmbedContents") {
				controller.RelayNotFound(c)
				return
			}
			controller.Relay(c, types.RelayFormatGemini)
		})
	}
}
