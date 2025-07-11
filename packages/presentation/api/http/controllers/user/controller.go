package usercontroller

import (
	"fmt"
	"net/http"
	"sentinel/packages/common/config"
	Error "sentinel/packages/common/errors"
	"sentinel/packages/common/validation"
	ActionDTO "sentinel/packages/core/action/DTO"
	"sentinel/packages/infrastructure/DB"
	"sentinel/packages/infrastructure/auth/authn"
	"sentinel/packages/infrastructure/auth/authz"
	"sentinel/packages/infrastructure/email"
	UserMapper "sentinel/packages/infrastructure/mappers/user"
	"sentinel/packages/infrastructure/token"
	controller "sentinel/packages/presentation/api/http/controllers"
	"sentinel/packages/presentation/api/http/request"
	RequestBody "sentinel/packages/presentation/data/request"
	ResponseBody "sentinel/packages/presentation/data/response"
	"strconv"
	"strings"

	rbac "github.com/StepanAnanin/SentinelRBAC"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func newActionDTO[T ActionDTO.Any](
	ctx echo.Context,
	uid string,
	mapFunc func (uid string, claims jwt.MapClaims) (T, *Error.Status),
) (T, *echo.HTTPError) {
	var zero T
    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Trace("Retrieving access token from the request...", reqMeta)

    accessToken, err := controller.GetAccessToken(ctx)
    if err != nil {
        controller.Logger.Error("Failed to retrieve valid access token from the request", err.Error(), reqMeta)
        return zero, controller.HandleTokenError(ctx, err)
    }

    controller.Logger.Trace("Retrieving access token from the request: OK", reqMeta)
    controller.Logger.Trace("Creating action DTO from token claims...", reqMeta)

	// claims can be trusted if token is valid
	act, err := mapFunc(uid, accessToken.Claims.(jwt.MapClaims))
    if err != nil {
        controller.Logger.Error("Failed to create action DTO from token claims", err.Error(), reqMeta)
        return zero, controller.ConvertErrorStatusToHTTP(err)
    }

    controller.Logger.Trace("Creating action DTO from token claims: OK", reqMeta)

    return act, nil
}

func newBasicActionDTO(ctx echo.Context) (*ActionDTO.Basic, *echo.HTTPError) {
	return newActionDTO(ctx, "", func (_ string, claims jwt.MapClaims) (*ActionDTO.Basic, *Error.Status) {
		return UserMapper.BasicActionDTOFromClaims(claims)
	})
}

func newTargetedActionDTO(ctx echo.Context, uid string) (*ActionDTO.Targeted, *echo.HTTPError) {
	return newActionDTO(ctx, uid, func (id string, claims jwt.MapClaims) (*ActionDTO.Targeted, *Error.Status) {
		return UserMapper.TargetedActionDTOFromClaims(id, claims)
	})
}

func Create(ctx echo.Context) error {
	var body RequestBody.LoginAndPassword

    if err := controller.BindAndValidate(ctx, &body); err != nil {
        return err
    }

    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info("Creating new user...", reqMeta)

    uid, err := DB.Database.Create(body.Login, body.Password)
    if err != nil {
        controller.Logger.Error("Failed to create new user", err.Error(), reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

    if config.App.IsLoginEmail {
        controller.Logger.Trace("Creating activation token...", reqMeta)

        tk, err := token.NewActivationToken(
            uid,
            body.Login,
            rbac.GetRolesNames(authz.Host.DefaultRoles),
        )
        if err != nil {
            controller.Logger.Error("Failed to create new activation token", err.Error(), reqMeta)
            return controller.ConvertErrorStatusToHTTP(err)
        }

        controller.Logger.Trace("Creating activation token: OK", reqMeta)
        controller.Logger.Trace("Creating and equeueing activation email...", reqMeta)

        err = email.CreateAndEnqueueActivationEmail(body.Login, tk.String())
        if err != nil {
            controller.Logger.Error("Failed to create and enqueue activation email", err.Error(), reqMeta)
            return controller.ConvertErrorStatusToHTTP(err)
        }

        controller.Logger.Trace("Creating and equeueing activation email: OK", reqMeta)
    }

    controller.Logger.Info("Creating new user: OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

type updater = func (*ActionDTO.Targeted) *Error.Status

// Updates user's state (deletion status).
// if omitUid is true, then uid will be set to empty string,
// otherwise uid will be taken from path params (in this case uid must be a valid UUID).
// If you want to change other user properties then use 'update' isntead.
func handleUserDeleteUpdate(ctx echo.Context, upd updater, omitUid bool, logMessageBase string) error {
    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info(logMessageBase + "...", reqMeta)

    var uid string

    if !omitUid {
        uid = ctx.Param("uid")
    }

    act, err := newTargetedActionDTO(ctx, uid)
    if err != nil {
		controller.Logger.Error(logMessageBase + ": FAILED", err.Error(), reqMeta)
        return err
    }

    if err := upd(act); err != nil {
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Info(logMessageBase + ": OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

func SoftDelete(ctx echo.Context) error {
    return handleUserDeleteUpdate(ctx, DB.Database.SoftDelete, false, "Soft deleting user")
}

func Restore(ctx echo.Context) error {
    return handleUserDeleteUpdate(ctx, DB.Database.Restore, false, "Restoring user")
}

func Drop(ctx echo.Context) error {
    return handleUserDeleteUpdate(ctx, DB.Database.Drop, false, "Dropping user")
}

func BulkSoftDelete(ctx echo.Context) error {
    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info("Bulk soft deleting users...", reqMeta)

    act, err := newBasicActionDTO(ctx)
    if err != nil {
		controller.Logger.Error("Bulk soft deleting users: FAILED", err.Error(), reqMeta)
        return err
    }

	var body RequestBody.UserIDs

	if e := controller.BindAndValidate(ctx, &body); e != nil {
		return e
	}

    if err := DB.Database.BulkSoftDelete(act, body.IDs); err != nil {
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Info("Bulk soft deleting users: OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

func BulkRestore(ctx echo.Context) error {
    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info("Bulk restoring users...", reqMeta)

    act, err := newBasicActionDTO(ctx)
    if err != nil {
		controller.Logger.Error("Bulk restoring users: FAILED", err.Error(), reqMeta)
        return err
    }

	var body RequestBody.UserIDs

	if e := controller.BindAndValidate(ctx, &body); e != nil {
		return e
	}

    if err := DB.Database.BulkRestore(act, body.IDs); err != nil {
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Info("Bulk restoring users: OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

func DropAllDeleted(ctx echo.Context) error {
    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info("Dropping all soft deleted user...", reqMeta)

    act, err := newTargetedActionDTO(ctx, "")
    if err != nil {
        return err
    }

    if err := DB.Database.DropAllSoftDeleted(&act.Basic); err != nil {
        controller.Logger.Error("Failed to drop all soft deleted user", err.Error(), reqMeta)

        return controller.ConvertErrorStatusToHTTP(err)
    }

    controller.Logger.Info("Dropping all soft deleted user: Ok", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

func validateUpdateRequestBody(filter *ActionDTO.Targeted, body RequestBody.UpdateUser) *echo.HTTPError {
    // if user tries to update himself
    if filter.RequesterUID == filter.TargetUID {
        if err := body.Validate(); err != nil {
            return echo.NewHTTPError(http.StatusBadRequest, err.Error())
        }

        user, err := DB.Database.FindAnyUserByID(filter.TargetUID)

        if err != nil {
            return controller.ConvertErrorStatusToHTTP(err)
        }

        if err := authn.CompareHashAndPassword(user.Password, body.GetPassword()); err != nil {
            return echo.NewHTTPError(err.Status(), "Неверный пароль")
        }

        return nil
    }

    // if user tries to update another user
    if err := body.Validate(); err != nil {
        if _, ok := body.(*RequestBody.ChangePassword); ok {
            if err == RequestBody.ErrorMissingPassword || err == RequestBody.ErrorInvalidPassword {
                return nil
            }
        }
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    return nil
}

// TODO try to find a way to merge 'update' and 'handleUserStateUpdate'

// Updates one of user's properties excluding state (deletion status).
// If you want to update user's state use 'handleUserStateUpdate' instead.
func update(ctx echo.Context, body RequestBody.UpdateUser, logMessageBase string) error {
    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info(logMessageBase + "...", reqMeta)

    controller.Logger.Trace("Binding request...", reqMeta)

    if err := ctx.Bind(body); err != nil {
        controller.Logger.Error("Failed to bind request", err.Error(), reqMeta)
        return err
    }

    controller.Logger.Trace("Binding request: OK", reqMeta)

    uid := ctx.Param("uid")

    act, e := newTargetedActionDTO(ctx, uid)
    if e != nil {
        return e
    }

    controller.Logger.Trace("Validating user update request...", reqMeta)

    if e := validateUpdateRequestBody(act, body); e != nil {
        controller.Logger.Error("Invalid user update request", e.Error(), reqMeta)
        return e
    }

    controller.Logger.Trace("Validating user update request: OK", reqMeta)

    var err *Error.Status

    switch b := body.(type) {
    case *RequestBody.ChangeLogin:
        err = DB.Database.ChangeLogin(act, b.Login)
    case *RequestBody.ChangePassword:
        err = DB.Database.ChangePassword(act, b.NewPassword)
    case *RequestBody.ChangeRoles:
        err = DB.Database.ChangeRoles(act, b.Roles)
    default:
		controller.Logger.Panic(
			"Invalid update call",
			fmt.Sprintf("Unexpected request body type - %T", body),
			reqMeta,
		)
        return nil
    }

    if err != nil {
		controller.Logger.Info(logMessageBase + ": FAILED", reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

	controller.Logger.Info(logMessageBase + ": OK", reqMeta)

    return ctx.NoContent(http.StatusOK)
}

func ChangeLogin(ctx echo.Context) error {
    return update(ctx, new(RequestBody.ChangeLogin), "Changing user login")
}

func ChangePassword(ctx echo.Context) error {
    return update(ctx, new(RequestBody.ChangePassword), "Changing user password")
}

func ChangeRoles(ctx echo.Context) error {
    return update(ctx, new(RequestBody.ChangeRoles), "Changing user roles")
}

func GetRoles(ctx echo.Context) error {
    uid := ctx.Param("uid")

    filter, e := newTargetedActionDTO(ctx, uid)
    if e != nil {
        return e
    }

    reqMeta := request.GetMetadata(ctx)

    controller.Logger.Info("Getting user roles...", reqMeta)

    roles, err := DB.Database.GetRoles(filter)
    if err != nil {
        controller.Logger.Error("Failed to get user roles", err.Error(), reqMeta)
        return controller.ConvertErrorStatusToHTTP(err)
    }

    controller.Logger.Info("Getting user roles: OK", reqMeta)

    return ctx.JSON(http.StatusOK, roles)
}

func IsLoginAvailable(ctx echo.Context) error {
    reqMeta := request.GetMetadata(ctx)

	login := ctx.QueryParam("login")

	controller.Logger.Info("Checking if login '"+login+"' available...", reqMeta)

    if login == "" {
		message := "query param 'login' isn't specified"

		controller.Logger.Error("Failed to check if login '"+login+"' available", message, reqMeta)

        return echo.NewHTTPError(
            http.StatusBadRequest,
			message,
        )
    }

    available := DB.Database.IsLoginAvailable(login)

	controller.Logger.Info(
		"Checking if login '"+login+"' available: " + strconv.FormatBool(available), reqMeta,
	)

    return ctx.JSON(
        http.StatusOK,
        ResponseBody.IsLoginAvailable{
            Available: available,
        },
    )
}

func SearchUsers(ctx echo.Context) error {
    reqMeta := request.GetMetadata(ctx)

	rawFilters := ctx.QueryParams()["filter"]
	rawPage := ctx.QueryParam("page")
	rawPageSize := ctx.QueryParam("pageSize")

	if rawFilters == nil || len(rawFilters) == 0 {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Filter is missing",
		)
	}
	if rawPage == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Query param 'page' is missing")
	}
	if rawPageSize == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Query param 'pageSize' is missing")
	}

	page, parseErr := strconv.Atoi(rawPage)
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "page must be an integer number")
	}
	pageSize, parseErr := strconv.Atoi(rawPageSize)
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "pageSize must be an integer number")
	}

	act, e := newBasicActionDTO(ctx)
	if e != nil {
		return e
	}

	controller.Logger.Info("Searching for users matching '"+strings.Join(rawFilters, ";")+"' filters...", reqMeta)

	dtos, err := DB.Database.SearchUsers(act, rawFilters, page, pageSize)
	if err != nil {
		return controller.ConvertErrorStatusToHTTP(err)
	}

	controller.Logger.Info("Searching for users matching '"+strings.Join(rawFilters, ";")+"' filters: OK", reqMeta)

	return ctx.JSON(http.StatusOK, dtos)
}

func GetUserSessions(ctx echo.Context) error {
	reqMeta := request.GetMetadata(ctx)

	uid := ctx.Param("uid")

	if e := validation.UUID(uid); e != nil {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			e.ToStatus(
				"User ID is missing in URL path",
				"User ID has invalid format (expected UUID)",
			).Error(),
		)
	}

	accessToken, err := controller.GetAccessToken(ctx)
	if err != nil {
		return controller.ConvertErrorStatusToHTTP(err)
	}

	payload, err := UserMapper.PayloadFromClaims(accessToken.Claims.(jwt.MapClaims))
	if err != nil {
		return controller.ConvertErrorStatusToHTTP(err)
	}

	controller.Logger.Info("Getting user sessions...", reqMeta)

	act := ActionDTO.NewTargeted(uid, payload.ID, payload.Roles)

	sessions, err := DB.Database.GetUserSessions(act)
	if err != nil {
		controller.Logger.Error("Failed to get user sessions", err.Error(), reqMeta)
		return controller.ConvertErrorStatusToHTTP(err)
	}

	controller.Logger.Info("Getting user sessions: OK", reqMeta)

	return ctx.JSON(http.StatusOK, sessions)
}

