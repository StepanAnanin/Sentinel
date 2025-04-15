package controller

import (
	"net/http"
	Error "sentinel/packages/common/errors"
	"time"

	"github.com/labstack/echo/v4"
)

func DeleteCookie(ctx echo.Context, cookie *http.Cookie) {
    cookie.Expires = time.Now().Add(time.Hour * -1)

    ctx.SetCookie(cookie)
}

func GetAuthCookie(ctx echo.Context) (*http.Cookie, *echo.HTTPError) {
    authCookie, err := ctx.Cookie(RefreshTokenCookieKey)

    if err != nil {
        if err == http.ErrNoCookie {
            return nil, ConvertErrorStatusToHTTP(Error.StatusUnauthorized)
        }
        return nil, ConvertErrorStatusToHTTP(Error.StatusInternalError)
    }

    return authCookie, nil
}

