package controller

import (
	"net/http"
	Error "sentinel/packages/common/errors"
	"sentinel/packages/presentation/api/http/request"
	"time"

	"github.com/labstack/echo/v4"
)

func DeleteCookie(ctx echo.Context, cookie *http.Cookie) {
    cookie.Expires = time.Now().Add(time.Hour * -1)

    ctx.SetCookie(cookie)
}

func GetAuthCookie(ctx echo.Context) (*http.Cookie, *echo.HTTPError) {
	reqMeta := request.GetMetadata(ctx)

	Logger.Trace("Getting auth cookie...", reqMeta)

    authCookie, err := ctx.Cookie(RefreshTokenCookieKey)

    if err != nil {
		Logger.Error("Failed to get auth cookie", err.Error(), reqMeta)

        if err == http.ErrNoCookie {
            return nil, ConvertErrorStatusToHTTP(Error.StatusUnauthorized)
        }
        return nil, ConvertErrorStatusToHTTP(Error.StatusInternalError)
    }

	Logger.Trace("Getting auth cookie: OK", reqMeta)

    return authCookie, nil
}

