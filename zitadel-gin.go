// Package zitadelgin provides a Gin middleware that authorizes incoming
// requests using the ZITADEL authorization SDK. It verifies the bearer token
// found in the request, optionally enforces role checks, and makes the
// resulting authorization context available to downstream handlers.
package zitadelgin

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
)

// authorizer is the subset of [authorization.Authorizer] used by the
// middleware. It is defined as an interface so the interceptor can be tested
// with a stub. *authorization.Authorizer[T] satisfies it.
type authorizer[T authorization.Ctx] interface {
	CheckAuthorization(ctx context.Context, token string, options ...authorization.CheckOption) (T, error)
}

// Interceptor wraps a ZITADEL [authorization.Authorizer] and exposes Gin
// middleware for protecting routes. T is the concrete authorization context
// type used by the configured authorizer.
type Interceptor[T authorization.Ctx] struct {
	authorizer authorizer[T]
}

// NewZitadelGin returns an [Interceptor] backed by the given authorizer.
func NewZitadelGin[T authorization.Ctx](authorizer *authorization.Authorizer[T]) *Interceptor[T] {
	return &Interceptor[T]{
		authorizer: authorizer,
	}
}

// RequireAuthorization returns a Gin middleware that verifies the request's
// authorization header before invoking the next handler. The optional
// [authorization.CheckOption] values (for example [authorization.WithRole])
// impose additional requirements such as a granted role.
//
// On success the verified authorization context is stored on the request
// context and can be retrieved with [Interceptor.Context]. On failure the
// request is aborted with an appropriate status code:
//
//   - 401 Unauthorized: the token is missing, malformed, or invalid.
//   - 403 Forbidden: the token is valid but lacks a required permission.
//   - 500 Internal Server Error: authorization could not be evaluated
//     (for example ZITADEL was unreachable). The underlying error is not
//     exposed to the caller.
func (i *Interceptor[T]) RequireAuthorization(options ...authorization.CheckOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		authCtx, err := i.authorizer.CheckAuthorization(c.Request.Context(), c.GetHeader(authorization.HeaderName), options...)
		if err != nil {
			switch {
			case errors.Is(err, &authorization.UnauthorizedErr{}):
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			case errors.Is(err, &authorization.PermissionDeniedErr{}):
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
			default:
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
			return
		}
		c.Request = c.Request.WithContext(authorization.WithAuthContext(c.Request.Context(), authCtx))
		c.Next()
	}
}

// Context returns the authorization context stored on ctx by
// [Interceptor.RequireAuthorization]. If no authorization context is present
// the zero value of T is returned.
func (i *Interceptor[T]) Context(ctx context.Context) T {
	return authorization.Context[T](ctx)
}
