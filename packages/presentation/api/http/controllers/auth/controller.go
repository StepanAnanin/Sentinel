package authcontroller

import (
	"net/http"
	Error "sentinel/packages/common/errors"
	UserDTO "sentinel/packages/core/user/DTO"
	"sentinel/packages/infrastructure/DB"
	"sentinel/packages/infrastructure/auth/authentication"
	UserMapper "sentinel/packages/infrastructure/mappers"
	"sentinel/packages/infrastructure/token"
	controller "sentinel/packages/presentation/api/http/controllers"
	datamodel "sentinel/packages/presentation/data"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func Login(ctx echo.Context) error {
    var body datamodel.LoginPasswordBody

    if err := controller.BindAndValidate(ctx, &body); err != nil {
        return err
    }

    user, err := DB.Database.FindAnyUserByLogin(body.Login)

    if err != nil {
        if err.Side() == Error.ClientSide {
            return echo.NewHTTPError(
                authentication.InvalidAuthCreditinals.Status(),
                authentication.InvalidAuthCreditinals.Error(),
            )
        }
        return controller.ConvertErrorStatusToHTTP(err)
    }

    if err := authentication.CompareHashAndPassword(user.Password, body.Password); err != nil {
        return controller.ConvertErrorStatusToHTTP(err)
    }

    accessToken, refreshToken := token.Generate(&UserDTO.Payload{
        ID: user.ID,
        Login: user.Login,
        Roles: user.Roles,
    })

    ctx.SetCookie(newAuthCookie(refreshToken))

    return ctx.JSON(
        http.StatusOK,
        datamodel.TokenResponseBody{
            Message: "Пользователь успешно авторизован",
            AccessToken: accessToken.Value,
            ExpiresIn: int(accessToken.TTL) / 1000,
        },
    )
}

func Logout(ctx echo.Context) error {
    authCookie, err := getAuthCookie(ctx)

    if err != nil {
        return err
    }

    deleteCookie(ctx, authCookie)

    return ctx.NoContent(http.StatusOK)
}

func Refresh(ctx echo.Context) error {
    authCookie, err := getAuthCookie(ctx)

    if err != nil {
        return err
    }

    oldRefreshToken, e := token.GetRefreshToken(authCookie)

    // if refresh token is either invalid or expired
    if e != nil {
        deleteCookie(ctx, authCookie)

        return controller.ConvertErrorStatusToHTTP(e)
    }

    payload, e := UserMapper.PayloadFromClaims(oldRefreshToken.Claims.(jwt.MapClaims))

    if e != nil {
        return controller.ConvertErrorStatusToHTTP(e)
    }

    accessToken, refreshToken := token.Generate(payload)

    ctx.SetCookie(newAuthCookie(refreshToken))

    return ctx.JSON(
        http.StatusOK,
        datamodel.TokenResponseBody{
            Message: "Токены успешно обновлены",
            AccessToken: accessToken.Value,
            ExpiresIn: int(accessToken.TTL) / 1000,
        },
    )
}

func Verify(ctx echo.Context) error {
    authHeader := ctx.Request().Header.Get("Authorization")

    accessToken, err := token.GetAccessToken(authHeader)

    if err != nil {
        return controller.ConvertErrorStatusToHTTP(err)
    }

    payload, err := UserMapper.PayloadFromClaims(accessToken.Claims.(jwt.MapClaims))

    if err != nil {
        return controller.ConvertErrorStatusToHTTP(err)
    }

    return ctx.JSON(http.StatusOK, payload)
}

