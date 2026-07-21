# ZITADEL gin's middleware

[![Go Reference](https://pkg.go.dev/badge/github.com/panapol-p/zitadel-gin.svg)](https://pkg.go.dev/github.com/panapol-p/zitadel-gin)
[![CI](https://github.com/panapol-p/zitadel-gin/actions/workflows/ci.yml/badge.svg)](https://github.com/panapol-p/zitadel-gin/actions/workflows/ci.yml)

[Gin](https://github.com/gin-gonic/gin) middleware for [ZITADEL](https://zitadel.com/).
It verifies the bearer token on incoming requests using the ZITADEL authorization
SDK, optionally enforces role checks, and exposes the authorization context to
your handlers.

## Install

```sh
go get github.com/panapol-p/zitadel-gin
```

## Usage

```go
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

	authZ, err := authorization.New(
		ctx,
		zitadel.New(os.Getenv("ZITADEL_DOMAIN")),
		oauth.DefaultAuthorization(os.Getenv("ZITADEL_KEY_PATH")),
	)
	if err != nil {
		slog.Error("zitadel sdk could not initialize", "error", err)
		os.Exit(1)
	}

	interceptor := zitadelgin.NewZitadelGin(authZ)

	r := gin.Default()

	// Any authenticated caller.
	protected := r.Group("/api/v1")
	protected.Use(interceptor.RequireAuthorization())
	protected.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "protected route"})
	})

	// Requires the "admin" role.
	admin := r.Group("/api/v1")
	admin.Use(interceptor.RequireAuthorization(authorization.WithRole("admin")))
	admin.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "admin route"})
	})

	if err := r.Run(); err != nil {
		slog.Error("gin server stopped", "error", err)
		os.Exit(1)
	}
}
```

A runnable version lives in [`example/`](./example).

## API

| Symbol | Description |
| ------ | ----------- |
| `NewZitadelGin(authorizer)` | Creates an `Interceptor` from a configured `*authorization.Authorizer[T]`. |
| `(*Interceptor).RequireAuthorization(options ...authorization.CheckOption)` | Returns a `gin.HandlerFunc` that verifies the request and applies the given checks (e.g. `authorization.WithRole("admin")`). |
| `(*Interceptor).Context(ctx)` | Returns the authorization context stored on the request by the middleware. |

### Responses

On failure the middleware aborts the request with a JSON body `{"error": "..."}`:

| Status | When |
| ------ | ---- |
| `401 Unauthorized` | Token is missing, malformed, or invalid. |
| `403 Forbidden` | Token is valid but lacks a required permission (e.g. role). |
| `500 Internal Server Error` | Authorization could not be evaluated (e.g. ZITADEL unreachable). The underlying error is not exposed to the caller. |

### Accessing the authorized user

Inside a protected handler, retrieve the authorization context:

```go
protected.GET("/me", func(c *gin.Context) {
	authCtx := interceptor.Context(c.Request.Context())
	c.JSON(200, gin.H{"user_id": authCtx.UserID()})
})
```

## Configuration

The example reads two environment variables:

| Variable | Description |
| -------- | ----------- |
| `ZITADEL_DOMAIN` | Your ZITADEL instance domain. |
| `ZITADEL_KEY_PATH` | Path to the API application key JSON file. |

See [`example/.env.example`](./example/.env.example).

## License

Released under the [MIT License](./LICENSE).
