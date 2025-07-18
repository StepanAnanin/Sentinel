package activationcontroller

import (
	"net/http"
	"sentinel/packages/infrastructure/DB"
	"sentinel/packages/infrastructure/email"
	"sentinel/packages/infrastructure/token"
	controller "sentinel/packages/presentation/api/http/controllers"
	"sentinel/packages/presentation/api/http/request"
	RequestBody "sentinel/packages/presentation/data/request"
	"strings"

	"github.com/labstack/echo/v4"
)

var tokenIsMissing = echo.NewHTTPError(
    http.StatusBadRequest,
    "Token is missing",
)

// @Summary 		Activate user
// @Description 	Activate user
// @ID 				activate
// @Tags			activation
// @Param 			token path string true "Activation token"
// @Accept			json
// @Produce			json
// @Success			200
// @Failure			400,401,500	{object} responsebody.Error
// @Router			/user/activate/{token} [get]
func Activate(ctx echo.Context) error {
	reqMeta := request.GetMetadata(ctx)

	controller.Logger.Info("Activating user...", nil)

    token := ctx.Param("token")

    if strings.ReplaceAll(token, " ", "") == "" {
		controller.Logger.Error("Failed to activate user", tokenIsMissing.Error(), reqMeta)
        return tokenIsMissing
    }

    if err := DB.Database.Activate(token); err != nil {
		controller.Logger.Error("Failed to activate user", err.Error(), reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Info("Activating user: OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

// @Summary 		Resend activation token
// @Description 	Create and send new activation token to user
// @ID 				resend-activation-token
// @Tags			activation
// @Param 			login body requestbody.UserLogin true "Login of not activated user to whom token should be sent"
// @Accept			json
// @Produce			json
// @Success			200
// @Failure			400,401,500	{object} responsebody.Error
// @Router			/user/activate/resend [put]
func Resend(ctx echo.Context) error {
	reqMeta := request.GetMetadata(ctx)

	controller.Logger.Info("Resending activation email...", reqMeta)

    var body RequestBody.UserLogin

    if e := controller.BindAndValidate(ctx, &body); e != nil {
		controller.Logger.Error("Failed to resend activation email", e.Error(), reqMeta)
        return e
    }

    user, err := DB.Database.FindUserByLogin(body.Login)
    if err != nil {
		controller.Logger.Error("Failed to resend activation email", err.Error(), reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

    if user.IsActive() {
		message := "User already active"

		controller.Logger.Error("Failed to resend activation email", message, reqMeta)

        return echo.NewHTTPError(
            http.StatusConflict,
            message,
        )
    }

	controller.Logger.Trace("Creating activation token...", reqMeta)

    tk, err := token.NewActivationToken(user.ID, user.Login, user.Roles)
    if err != nil {
		controller.Logger.Error("Failed to create activation token", err.Error(), reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Trace("Creating activation token...", reqMeta)
	controller.Logger.Trace("Creating and enqueueing activation email", reqMeta)

	err = email.CreateAndEnqueueActivationEmail(user.Login, tk.String())
    if err != nil {
		controller.Logger.Error("Failed to create and enqueue activation email", err.Error(), reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Trace("Creating and enqueueing activation email: OK", reqMeta)
	controller.Logger.Info("Resending activation email: OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

