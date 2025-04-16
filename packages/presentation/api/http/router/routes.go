package router

import (
	"net/http"
	"sentinel/packages/common/config"
	Activation "sentinel/packages/presentation/api/http/controllers/activation"
	Auth "sentinel/packages/presentation/api/http/controllers/auth"
	Cache "sentinel/packages/presentation/api/http/controllers/cache"
	Roles "sentinel/packages/presentation/api/http/controllers/roles"
	User "sentinel/packages/presentation/api/http/controllers/user"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// i could just explicitly pass empty string in routes when i need it
// but it looks really awful, shitty and not obvious
const rootPath = ""

func Create() *echo.Echo {
	router := echo.New()

    router.HideBanner = true
    router.HidePort = true

    router.HTTPErrorHandler = handleHttpError
    router.JSONSerializer = serializer{}
    router.Binder = &binder{}

    cors := middleware.CORSConfig{
        Skipper:      middleware.DefaultSkipper,
        AllowOrigins: config.HTTP.AllowedOrigins,
        AllowCredentials: true,
        AllowMethods: []string{
            http.MethodGet,
            http.MethodHead,
            http.MethodPut,
            http.MethodPatch,
            http.MethodPost,
            http.MethodDelete,
        },
    }

    router.Use(middleware.CORSWithConfig(cors))
    router.Use(middleware.Recover())
    // router.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(10_000)))

    if config.Debug.Enabled {
        router.Use(middleware.Logger())
    }

    authGroup := router.Group("/auth")

    authGroup.GET(rootPath, Auth.Verify)
    authGroup.POST(rootPath, Auth.Login)
    authGroup.PUT(rootPath, Auth.Refresh)
    authGroup.DELETE(rootPath, Auth.Logout)

    userGroup := router.Group("/user")

    userGroup.POST(rootPath, User.Create)
    userGroup.DELETE("/:uid", User.SoftDelete)
    userGroup.POST("/:uid/restore", User.Restore)
    userGroup.DELETE("/:uid/drop", User.Drop)
    userGroup.DELETE("/all/drop", User.DropAllDeleted)
    userGroup.POST("/login/available", User.IsLoginAvailable)
    userGroup.GET("/:uid/roles", User.GetRoles)
    userGroup.PATCH("/:uid/login", User.ChangeLogin)
    userGroup.PATCH("/:uid/password", User.ChangePassword)
    userGroup.PATCH("/:uid/roles", User.ChangeRoles)
    userGroup.GET("/activation/:token", Activation.Activate)
    userGroup.PUT("/activation/resend", Activation.Resend)

    rolesGroup := router.Group("/roles")

    rolesGroup.GET("/:serviceID", Roles.GetAll)

    cacheGroup := router.Group("/cache")

    cacheGroup.DELETE(rootPath, Cache.Drop)

    return router
}

