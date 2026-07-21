package zitadelgin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
)

// fakeCtx is a minimal implementation of [authorization.Ctx] used in tests.
type fakeCtx struct {
	authorized bool
	userID     string
	token      string
}

func (c *fakeCtx) IsAuthorized() bool                                 { return c.authorized }
func (c *fakeCtx) OrganizationID() string                             { return "" }
func (c *fakeCtx) UserID() string                                     { return c.userID }
func (c *fakeCtx) IsGrantedRole(string) bool                          { return false }
func (c *fakeCtx) IsGrantedRoleInProject(string, string, string) bool { return false }
func (c *fakeCtx) IsGrantedRoleInOrganization(string, string) bool    { return false }
func (c *fakeCtx) SetToken(token string)                              { c.token = token }
func (c *fakeCtx) GetToken() string                                   { return c.token }

// fakeAuthorizer is a stub authorizer that returns preconfigured results.
type fakeAuthorizer struct {
	ctx *fakeCtx
	err error
}

func (a *fakeAuthorizer) CheckAuthorization(context.Context, string, ...authorization.CheckOption) (*fakeCtx, error) {
	return a.ctx, a.err
}

func newTestRouter(a authorizer[*fakeCtx]) (*gin.Engine, *Interceptor[*fakeCtx]) {
	gin.SetMode(gin.TestMode)
	i := &Interceptor[*fakeCtx]{authorizer: a}
	r := gin.New()
	return r, i
}

func TestRequireAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		authorizer *fakeAuthorizer
		wantStatus int
	}{
		{
			name:       "authorized",
			authorizer: &fakeAuthorizer{ctx: &fakeCtx{authorized: true, userID: "user-1"}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unauthorized",
			authorizer: &fakeAuthorizer{err: authorization.NewErrorUnauthorized(errors.New("bad token"))},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "permission denied",
			authorizer: &fakeAuthorizer{err: authorization.NewErrorPermissionDenied(errors.New("missing role"))},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "internal error",
			authorizer: &fakeAuthorizer{err: errors.New("zitadel unreachable")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, i := newTestRouter(tt.authorizer)
			r.GET("/protected", i.RequireAuthorization(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestInternalErrorDoesNotLeak ensures the underlying error message is not
// exposed to clients on a 500 response.
func TestInternalErrorDoesNotLeak(t *testing.T) {
	secret := "zitadel unreachable at 10.0.0.5"
	r, i := newTestRouter(&fakeAuthorizer{err: errors.New(secret)})
	r.GET("/protected", i.RequireAuthorization(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if body := w.Body.String(); strings.Contains(body, secret) {
		t.Fatalf("response leaked internal error: %s", body)
	}
}

// TestAuthContextPropagated ensures the verified authorization context is made
// available to downstream handlers via Interceptor.Context.
func TestAuthContextPropagated(t *testing.T) {
	want := &fakeCtx{authorized: true, userID: "user-42"}
	r, i := newTestRouter(&fakeAuthorizer{ctx: want})

	var gotUserID string
	r.GET("/protected", i.RequireAuthorization(), func(c *gin.Context) {
		gotUserID = i.Context(c.Request.Context()).UserID()
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if gotUserID != "user-42" {
		t.Fatalf("propagated user id = %q, want %q", gotUserID, "user-42")
	}
}
