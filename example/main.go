package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	zitadelgin "github.com/panapol-p/zitadel-gin"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization/oauth"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

func main() {
	ctx := context.Background()
	authZ, err := authorization.New(ctx, zitadel.New(os.Getenv("ZITADEL_DOMAIN")), oauth.DefaultAuthorization(os.Getenv("ZITADEL_KEY_PATH")))
	if err != nil {
		slog.Error("zitadel sdk could not initialize", "error", err)
		os.Exit(1)
	}
	zitadelInterceptor := zitadelgin.NewZitadelGin(authZ)

	r := gin.Default()
	protectRoute := r.Group("/api/v1")
	protectRoute.Use(zitadelInterceptor.RequireAuthorization())
	protectRoute.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "protected route",
		})
	})

	adminRoute := r.Group("/api/v1")
	adminRoute.Use(zitadelInterceptor.RequireAuthorization(authorization.WithRole("admin")))
	adminRoute.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "admin route",
		})
	})

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	if err = r.Run(); err != nil {
		slog.Error("")
	}
}
