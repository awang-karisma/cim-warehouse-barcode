package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/adapter"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/session"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/auth"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/version"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
)

type contextKey string

const authClaimsKey contextKey = "auth_claims"

func claimsFromContext(ctx context.Context) *auth.Claims {
	if ctx == nil {
		return nil
	}
	if claims, ok := ctx.Value(authClaimsKey).(*auth.Claims); ok {
		return claims
	}
	return nil
}

func (s *Server) requirePermission(c fiber.Ctx, perm string) error {
	if s.appConfig == nil || s.appConfig.GoogleClientID == "" {
		return nil
	}
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	// Superadmin always passes
	if model.UserRole(claims.Role) == model.RoleSuperadmin || (s.appConfig != nil && s.appConfig.IsSuperadmin(claims.Email)) {
		return nil
	}
	// Check permission list
	for _, p := range claims.Permissions {
		if p == perm {
			return nil
		}
	}
	return fiber.NewError(fiber.StatusForbidden, "insufficient permissions")
}

func (s *Server) resolvePermissions(ctx context.Context, roleName string) ([]string, error) {
	if s.permCache != nil {
		if perms, ok := s.permCache.Get(roleName); ok {
			return perms, nil
		}
	}
	perms, err := s.store.GetPermissionsByRoleName(ctx, roleName)
	if err != nil {
		return nil, err
	}
	if s.permCache != nil {
		s.permCache.Set(roleName, perms)
	}
	return perms, nil
}

// Config holds dependencies for creating an API Server.
type Config struct {
	Store        *storage.Storage
	AppConfig    *config.Config
	PermCache    rbac.PermissionCache
	AccuratePool *accounts.Pool
}

// Server wraps the Fiber v3 HTTP application and Huma v2 API.
type Server struct {
	app                    *fiber.App
	humaAPI                huma.API
	store                  *storage.Storage
	appConfig              *config.Config
	permCache              rbac.PermissionCache
	accurateProvider       accurate.Provider
	accurateSession        *session.Manager
	accurateRegistry       *adapter.Registry
	accountPool            *accounts.Pool
	accountEmail           string
	startTime              time.Time
	accountOperationUnlock func()
}

// New creates and initializes a new Fiber v3 API Server with middlewares and routes.
func New(cfg Config) *Server {
	app := fiber.New(fiber.Config{
		AppName:   "Warehouse Barcode API",
		BodyLimit: 10 * 1024 * 1024,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})
	// Middlewares
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	humaCfg := huma.DefaultConfig("Warehouse Barcode API", version.Version)
	humaCfg.DocsRenderer = huma.DocsRendererScalar
	humaCfg.DocsPath = "/docs"
	humaCfg.OpenAPIPath = "/openapi"
	humaCfg.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", func(t reflect.Type, hint string) string {
		name := huma.DefaultSchemaNamer(t, hint)
		elem := t
		for elem != nil && (elem.Kind() == reflect.Ptr || elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array) {
			elem = elem.Elem()
		}
		if elem != nil && strings.Contains(elem.PkgPath(), "pkg/accurate") && !strings.HasPrefix(name, "Accurate") {
			return "Accurate" + name
		}
		return name
	})
	humaAPI := humafiber.New(app, humaCfg)

	permCache := cfg.PermCache
	if permCache == nil {
		permCache = rbac.NewMemoryCache(5 * time.Minute)
	}

	s := &Server{
		app:         app,
		humaAPI:     humaAPI,
		store:       cfg.Store,
		appConfig:   cfg.AppConfig,
		permCache:   permCache,
		startTime:   time.Now(),
		accountPool: cfg.AccuratePool,
	}
	if s.accountPool == nil {
		s.accountPool = accounts.NewPool(s.store, nil)
	}
	app.Use(s.authMiddleware)
	humaAPI.UseMiddleware(s.humaAuthMiddleware)

	s.registerOperations()

	// Auth routes (raw Fiber routes)
	app.Get("/auth/config", s.handleAuthConfig)
	app.Post("/auth/google/callback", s.handleGoogleCallback)
	app.Get("/auth/me", s.handleAuthMe)
	app.Post("/auth/logout", s.handleLogout)

	// Admin routes (raw Fiber routes)
	app.Get("/api/v1/admin/users", s.handleListUsers)
	app.Put("/api/v1/admin/users/:email/role", s.handleSetUserRole)
	app.Delete("/api/v1/admin/users/:email", s.handleDeleteUser)

	// Admin role routes (raw Fiber routes)
	app.Get("/api/v1/admin/roles", s.handleListRoles)
	app.Post("/api/v1/admin/roles", s.handleCreateRole)
	app.Put("/api/v1/admin/roles/:id", s.handleUpdateRole)
	app.Delete("/api/v1/admin/roles/:id", s.handleDeleteRole)
	app.Get("/api/v1/admin/roles/:id/permissions", s.handleGetRolePermissions)
	app.Put("/api/v1/admin/roles/:id/permissions", s.handleSetRolePermissions)
	app.Get("/api/v1/admin/permissions", s.handleListPermissions)

	return s
}

// App returns the underlying Fiber app instance.
func (s *Server) App() *fiber.App {
	return s.app
}

// HumaAPI returns the underlying Huma API instance.
func (s *Server) HumaAPI() huma.API {
	return s.humaAPI
}

// SetupStaticFiles configures static asset serving and SPA fallback to index.html.
func (s *Server) SetupStaticFiles(distPath string) {
	s.app.Use("/assets", static.New(filepath.Join(distPath, "assets"), static.Config{
		ModifyResponse: func(c fiber.Ctx) error {
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
			return nil
		},
	}))
	s.app.Get("/*", func(c fiber.Ctx) error {
		p := c.Path()
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/health") || strings.HasPrefix(p, "/docs") || strings.HasPrefix(p, "/openapi") || strings.HasPrefix(p, "/auth") {
			return c.Next()
		}
		if strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/assets") {
			return c.Status(fiber.StatusNotFound).SendString("Asset not found")
		}
		targetFile := filepath.Join(distPath, filepath.Clean(p))
		if info, err := os.Stat(targetFile); err == nil && !info.IsDir() {
			if strings.HasSuffix(targetFile, ".html") {
				c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Set("Pragma", "no-cache")
				c.Set("Expires", "0")
			}
			return c.SendFile(targetFile)
		}
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")
		return c.SendFile(filepath.Join(distPath, "index.html"))
	})
}

// SetupStaticFS configures static asset serving and SPA fallback using an in-memory or embedded fs.FS.
func (s *Server) SetupStaticFS(distFS fs.FS) {
	s.app.Use("/", static.New("", static.Config{
		FS:         distFS,
		IndexNames: []string{"index.html"},
		ModifyResponse: func(c fiber.Ctx) error {
			p := c.Path()
			if strings.HasPrefix(p, "/assets/") {
				c.Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if p == "/" || strings.HasSuffix(p, ".html") {
				c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Set("Pragma", "no-cache")
				c.Set("Expires", "0")
			}
			return nil
		},
	}))
	s.app.Get("/*", func(c fiber.Ctx) error {
		p := c.Path()
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/health") || strings.HasPrefix(p, "/docs") || strings.HasPrefix(p, "/openapi") || strings.HasPrefix(p, "/auth") {
			return c.Next()
		}
		if strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/assets") {
			return c.Status(fiber.StatusNotFound).SendString("Asset not found")
		}
		indexData, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Frontend build not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")
		return c.Status(fiber.StatusOK).Send(indexData)
	})
}

func (s *Server) registerOperations() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Description: "Check the health and readiness of the API service",
		Tags:        []string{"system"},
	}, s.handleHealth)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-system-info",
		Method:      http.MethodGet,
		Path:        "/api/v1/system",
		Summary:     "Get system diagnostics",
		Description: "Get runtime stats, uptime, port, db path, and SQLite table metrics",
		Tags:        []string{"system"},
		Metadata:    map[string]any{"permission": string(model.PermSystemRead)},
	}, s.handleSystem)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "set-user-locale",
		Method:      http.MethodPut,
		Path:        "/api/v1/user/locale",
		Summary:     "Update user locale",
		Description: "Persist the current user's preferred language locale ('en' or 'id')",
		Tags:        []string{"user"},
		Metadata:    map[string]any{"authenticated": true},
	}, s.handleSetUserLocale)

	// Custom Page Designs Endpoints
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-page-designs",
		Method:      http.MethodGet,
		Path:        "/api/v1/page-designs",
		Summary:     "List page designs",
		Description: "Get all custom page designs for barcode label printing",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermDesignRead)},
	}, s.handleGetPageDesigns)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-page-design",
		Method:      http.MethodPost,
		Path:        "/api/v1/page-designs",
		Summary:     "Create page design",
		Description: "Create a new custom page design for barcode label printing",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermDesignCreate)},
	}, s.handleCreatePageDesign)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "update-page-design",
		Method:      http.MethodPut,
		Path:        "/api/v1/page-designs/{id}",
		Summary:     "Update page design",
		Description: "Update an existing custom page design",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermDesignUpdate)},
	}, s.handleUpdatePageDesign)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "delete-page-design",
		Method:      http.MethodDelete,
		Path:        "/api/v1/page-designs/{id}",
		Summary:     "Delete page design",
		Description: "Delete a custom page design",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermDesignDelete)},
	}, s.handleDeletePageDesign)

	// Label Layout Presets Endpoints
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-label-layout-presets",
		Method:      http.MethodGet,
		Path:        "/api/v1/label-layout-presets",
		Summary:     "List label layout presets",
		Description: "List reusable barcode label element arrangements",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermPresetRead)},
	}, s.handleGetLabelLayoutPresets)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-label-layout-preset",
		Method:      http.MethodPost,
		Path:        "/api/v1/label-layout-presets",
		Summary:     "Create label layout preset",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermPresetCreate)},
	}, s.handleCreateLabelLayoutPreset)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "update-label-layout-preset",
		Method:      http.MethodPut,
		Path:        "/api/v1/label-layout-presets/{id}",
		Summary:     "Update label layout preset",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermPresetUpdate)},
	}, s.handleUpdateLabelLayoutPreset)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "delete-label-layout-preset",
		Method:      http.MethodDelete,
		Path:        "/api/v1/label-layout-presets/{id}",
		Summary:     "Delete label layout preset",
		Tags:        []string{"barcode"},
		Metadata:    map[string]any{"permission": string(model.PermPresetDelete)},
	}, s.handleDeleteLabelLayoutPreset)

	// Accurate account credential management uses a separate RBAC resource.
	huma.Register(s.humaAPI, huma.Operation{OperationID: "list-accurate-accounts", Method: http.MethodGet, Path: "/api/v1/accurate/accounts", Summary: "List Accurate accounts", Tags: []string{"accurate"}, Metadata: map[string]any{"permission": string(model.PermAccurateAccountRead)}}, s.handleListAccurateAccounts)
	huma.Register(s.humaAPI, huma.Operation{OperationID: "create-accurate-account", Method: http.MethodPost, Path: "/api/v1/accurate/accounts", Summary: "Create Accurate account", Tags: []string{"accurate"}, DefaultStatus: http.StatusCreated, Metadata: map[string]any{"permission": string(model.PermAccurateAccountCreate)}}, s.handleCreateAccurateAccount)
	huma.Register(s.humaAPI, huma.Operation{OperationID: "update-accurate-account", Method: http.MethodPut, Path: "/api/v1/accurate/accounts/{accountId}", Summary: "Update Accurate account", Tags: []string{"accurate"}, Metadata: map[string]any{"permission": string(model.PermAccurateAccountUpdate)}}, s.handleUpdateAccurateAccount)
	huma.Register(s.humaAPI, huma.Operation{OperationID: "delete-accurate-account", Method: http.MethodDelete, Path: "/api/v1/accurate/accounts/{accountId}", Summary: "Delete Accurate account", Tags: []string{"accurate"}, Metadata: map[string]any{"permission": string(model.PermAccurateAccountDelete)}}, s.handleDeleteAccurateAccount)
	huma.Register(s.humaAPI, huma.Operation{OperationID: "activate-accurate-account", Method: http.MethodPost, Path: "/api/v1/accurate/accounts/{accountId}/activate", Summary: "Activate Accurate account", Tags: []string{"accurate"}, Metadata: map[string]any{"permission": string(model.PermAccurateAccountUpdate)}}, s.handleActivateAccurateAccount)

	// Accurate Online Integration Endpoints
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-accurate-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/accurate/status",
		Summary:     "Get Accurate Online integration status",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateRead)},
	}, s.handleAccurateStatus)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "accurate-login",
		Method:      http.MethodPost,
		Path:        "/api/v1/accurate/login",
		Summary:     "Login or submit MFA for Accurate Online",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateCreate)},
	}, s.handleAccurateLogin)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "accurate-logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/accurate/logout",
		Summary:     "Log out from Accurate Online session",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateDelete)},
	}, s.handleAccurateLogout)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-accurate-databases",
		Method:      http.MethodGet,
		Path:        "/api/v1/accurate/databases",
		Summary:     "List accessible Accurate databases",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateRead)},
	}, s.handleAccurateDatabases)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "select-accurate-database",
		Method:      http.MethodPost,
		Path:        "/api/v1/accurate/database",
		Summary:     "Select and open Accurate database",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateUpdate)},
	}, s.handleAccurateSelectDatabase)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "search-accurate-items",
		Method:      http.MethodGet,
		Path:        "/api/v1/accurate/items",
		Summary:     "Search items from Accurate inventory",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateRead)},
	}, s.handleAccurateItems)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-accurate-sources",
		Method:      http.MethodGet,
		Path:        "/api/v1/accurate/sources",
		Summary:     "List registered Accurate transaction sources",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateRead)},
	}, s.handleAccurateSources)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-accurate-source-documents",
		Method:      http.MethodGet,
		Path:        "/api/v1/accurate/sources/{source}/documents",
		Summary:     "List documents for an Accurate transaction source",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateRead)},
	}, s.handleAccurateSourceDocuments)

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-accurate-source-document-items",
		Method:      http.MethodGet,
		Path:        "/api/v1/accurate/sources/{source}/documents/{id}/items",
		Summary:     "Get line items for an Accurate source document",
		Tags:        []string{"accurate"},
		Metadata:    map[string]any{"permission": string(model.PermAccurateRead)},
	}, s.handleAccurateSourceDocumentItems)
}

// clearSessionCookie expires the warehouse_session cookie.
func clearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "warehouse_session",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Lax",
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
}

// resolveCurrentUser looks up the user by email in the database and returns a copy of claims
// with the latest email, name, picture, and effective role.
func (s *Server) resolveCurrentUser(ctx context.Context, claims *auth.Claims) (*auth.Claims, bool, error) {
	if claims == nil {
		return nil, false, nil
	}
	if s.store == nil {
		return nil, false, errors.New("storage service not available")
	}
	user, err := s.store.GetUserByEmail(ctx, claims.Email)
	if err != nil {
		return nil, false, err
	}
	if user == nil {
		return nil, false, nil
	}

	refreshed := *claims
	refreshed.Email = user.Email
	refreshed.Name = user.Name
	refreshed.Picture = user.Picture
	refreshed.Role = user.Role
	refreshed.Locale = user.Locale
	if s.appConfig != nil && s.appConfig.IsSuperadmin(user.Email) {
		refreshed.Role = "superadmin"
		refreshed.Permissions = nil
	} else {
		perms, err := s.resolvePermissions(ctx, refreshed.Role)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, false, err
		}
		refreshed.Permissions = perms
	}

	return &refreshed, true, nil
}

// authMiddleware verifies the warehouse_session cookie for protected routes.
func (s *Server) authMiddleware(c fiber.Ctx) error {
	if s.appConfig == nil || s.appConfig.GoogleClientID == "" {
		return c.Next()
	}

	p := c.Path()
	if strings.HasPrefix(p, "/auth/") || p == "/auth" ||
		p == "/health" || strings.HasPrefix(p, "/health/") ||
		strings.HasPrefix(p, "/docs") ||
		strings.HasPrefix(p, "/openapi") ||
		(!strings.HasPrefix(p, "/api/") && p != "/api") {
		return c.Next()
	}

	cookie := c.Cookies("warehouse_session")
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	claims, err := auth.ValidateToken(s.appConfig.JWTSecret, cookie)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	refreshedClaims, found, err := s.resolveCurrentUser(c.Context(), claims)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to resolve user: %v", err)})
	}
	if !found {
		clearSessionCookie(c)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	c.Locals("user", refreshedClaims)
	c.SetContext(context.WithValue(c.Context(), authClaimsKey, refreshedClaims))
	return c.Next()
}

// humaAuthMiddleware enforces permission authorization for Huma operations.
func (s *Server) humaAuthMiddleware(ctx huma.Context, next func(huma.Context)) {
	if s.appConfig == nil || s.appConfig.GoogleClientID == "" {
		next(ctx)
		return
	}

	op := ctx.Operation()
	if op == nil {
		next(ctx)
		return
	}

	if !strings.HasPrefix(op.Path, "/api/v1/") {
		next(ctx)
		return
	}

	claims := claimsFromContext(ctx.Context())
	if claims == nil {
		_ = huma.WriteErr(s.humaAPI, ctx, http.StatusUnauthorized, "authentication required")
		return
	}

	// Superadmin always bypasses
	if model.UserRole(claims.Role) == model.RoleSuperadmin || (s.appConfig != nil && s.appConfig.IsSuperadmin(claims.Email)) {
		next(ctx)
		return
	}

	// Guest has no permissions and is forbidden from protected operations
	if model.UserRole(claims.Role) == model.RoleGuest {
		_ = huma.WriteErr(s.humaAPI, ctx, http.StatusForbidden, "insufficient permissions")
		return
	}

	if op.Metadata != nil {
		if requiredPerm, ok := op.Metadata["permission"].(string); ok && requiredPerm != "" {
			hasPerm := false
			for _, p := range claims.Permissions {
				if p == requiredPerm {
					hasPerm = true
					break
				}
			}
			if !hasPerm {
				_ = huma.WriteErr(s.humaAPI, ctx, http.StatusForbidden, "insufficient permissions")
				return
			}
		}
	}

	next(ctx)
}
