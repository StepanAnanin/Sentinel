package postgres

import (
	"net/http"
	"sentinel/packages/common/config"
	Error "sentinel/packages/common/errors"
	"sentinel/packages/core/user"
	UserDTO "sentinel/packages/core/user/DTO"
	"sentinel/packages/infrastructure/auth/authorization"
	"sentinel/packages/infrastructure/cache"
	UserMapper "sentinel/packages/infrastructure/mappers/user"
	"sentinel/packages/infrastructure/token"
	"slices"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type repository struct {
    //
}

func (_ *repository) checkLogin(login string) *Error.Status {
    if err := user.VerifyLogin(login); err != nil {
        return err
    }

    _, err := driver.FindAnyUserByLogin(login)

    if err != nil {
        // user wasn't found, hence login is free to use
        if err.Status() == http.StatusNotFound {
            return nil
        }

        return err
    }

    // if there are no any error (which means that user with this login exists)
    return loginAlreadyInUse
}

func (r *repository) Create(login string, password string) (string, *Error.Status) {
    if err := r.checkLogin(login); err != nil {
        return "", err
    }

	hashedPassword, err := hashPassword(password)

    if err != nil {
        return "", nil
    }

    uid := uuid.New()

    query := newQuery(
        `INSERT INTO "user" (id, login, password, roles) VALUES
        ($1, $2, $3, $4);`,
        uid, login, hashedPassword, []string{authorization.Host.OriginRoleName},
    )

    if err = cache.Client.DeleteOnError(
        query.Exec(),
        // TODO try to replace that everywhere with cache.client.DeletePattern
        cache.KeyBase[cache.UserByLogin] + login,
        cache.KeyBase[cache.AnyUserByLogin] + login,
    ); err != nil {
        return "", err
    }

    return uid.String(), nil
}

func (_ *repository) SoftDelete(filter *UserDTO.Filter) *Error.Status {
    // TODO add possibility to config what kind of users can delete themselves
    // all users can delete themselves, except admins (TEMP)
    if filter.TargetUID != filter.RequesterUID {
        if err := authorization.Authorize(
            authorization.Action.SoftDelete,
            authorization.Resource.User,
            filter.RequesterRoles,
        ); err != nil {
            return err
        }
    }

    user, err := driver.FindUserByID(filter.TargetUID)

    if err != nil {
        return err
    }

    if slices.Contains(user.Roles, "admin") {
        return Error.NewStatusError(
            "Нельзя удалить пользователя с ролью администратора",
            http.StatusBadRequest,
        )
    }

    audit := newAudit(deleteOperation, filter, user)

    query := newQuery(
        `UPDATE "user" SET deleted_at = $1
        WHERE id = $2 AND deleted_at IS NULL;`,
        // deleted_at set manualy instead of using NOW()
        // cuz changed_at and deleted_at should be synchronized
        audit.ChangedAt, filter.TargetUID,
    )

    return cache.Client.DeleteOnError(
        execWithAudit(&audit, query),
        cache.KeyBase[cache.UserById] + filter.TargetUID,
        cache.KeyBase[cache.DeletedUserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserRolesById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    )
}

func (_ *repository) Restore(filter *UserDTO.Filter) *Error.Status {
    if err := authorization.Authorize(
        authorization.Action.Restore,
        authorization.Resource.User,
        filter.RequesterRoles,
    ); err != nil {
        return err
    }

    user, err := driver.FindSoftDeletedUserByID(filter.TargetUID)

    if err != nil {
        return err
    }

    audit := newAudit(restoreOperation, filter, user)

    query := newQuery(
        `UPDATE "user" SET deleted_at = NULL
        WHERE id = $1 AND deleted_at IS NOT NULL;`,
        filter.TargetUID,
    )

    return cache.Client.DeleteOnError(
        execWithAudit(&audit, query),
        cache.KeyBase[cache.UserById] + filter.TargetUID,
        cache.KeyBase[cache.DeletedUserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserRolesById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    )
}

// TODO add audit (there are some problem with foreign keys)
func (_ *repository) Drop(filter *UserDTO.Filter) *Error.Status {
    if err := authorization.Authorize(
        authorization.Action.Drop,
        authorization.Resource.User,
        filter.RequesterRoles,
    ); err != nil {
        return err
    }

    user, err := driver.FindAnyUserByID(filter.TargetUID)

    if err != nil {
        return err
    }

    if user.DeletedAt.IsZero() {
        return Error.NewStatusError(
            "Only soft deleted users can be dropped",
            http.StatusBadRequest,
        )
    }

    query := newQuery(
        `DELETE FROM "user"
        WHERE id = $1 AND deleted_at IS NOT NULL;`,
        filter.TargetUID,
    )

    return cache.Client.DeleteOnError(
        query.Exec(),
        cache.KeyBase[cache.DeletedUserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserRolesById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    )
}

// TODO add audit (this method really cause a lot of problems)
func (_ *repository) DropAllSoftDeleted(filter *UserDTO.Filter) *Error.Status {
    if err := authorization.Authorize(
        authorization.Action.DropAllSoftDeleted,
        authorization.Resource.User,
        filter.RequesterRoles,
    ); err != nil {
        return err
    }

    query := newQuery(
        `DELETE FROM "user"
        WHERE deleted_at IS NOT NULL;`,
    )

    err := query.Exec()

    cache.Client.ProgressiveDeletePattern(cache.DeletedUserKeyPrefix + "*")

    return err
}

func (r *repository) ChangeLogin(filter *UserDTO.Filter, newLogin string) *Error.Status {
    if err := authorization.Authorize(
        authorization.Action.ChangeLogin,
        authorization.Resource.User,
        filter.RequesterRoles,
    ); err != nil {
        return err
    }

    user, err := driver.FindUserByID(filter.TargetUID)

    if err != nil {
        return err
    }

    if user.Login == newLogin {
        return Error.NewStatusError(
            "Login not changed: Current login and new login are the same",
            http.StatusConflict,
        )
    }

    if err := r.checkLogin(newLogin); err != nil {
        return err
    }

    audit := newAudit(updatedOperation, filter, user)

    query := newQuery(
        `UPDATE "user" SET login = $1
        WHERE id = $2;`,
        newLogin, filter.TargetUID,
    )

    return cache.Client.DeleteOnError(
        execWithAudit(&audit, query),
        cache.KeyBase[cache.UserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    )
}

func (_ *repository) ChangePassword(filter *UserDTO.Filter, newPassword string) *Error.Status {
    if filter.RequesterUID != filter.TargetUID {
        if err := authorization.Authorize(
            authorization.Action.ChangePassword,
            authorization.Resource.User,
            filter.RequesterRoles,
        ); err != nil {
            return err
        }
    }

    user, err := driver.FindUserByID(filter.TargetUID)

    if err != nil {
        return err
    }

	hashedPassword, e := hashPassword(newPassword)

    if e != nil {
        return e
    }

    audit := newAudit(updatedOperation, filter, user)

    query := newQuery(
        `UPDATE "user" SET password = $1
        WHERE id = $2;`,
        hashedPassword, filter.TargetUID,
    )

    return cache.Client.DeleteOnError(
        execWithAudit(&audit, query),
        cache.KeyBase[cache.UserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    )
}

func (_ *repository) ChangeRoles(filter *UserDTO.Filter, newRoles []string) *Error.Status {
    if filter.TargetUID == filter.RequesterUID &&
       slices.Contains(filter.RequesterRoles, "admin") &&
       !slices.Contains(newRoles, "admin") {
          return Error.NewStatusError(
              "Нельзя снять роль администратора с самого себя",
               http.StatusForbidden,
          )
    }

    user, err := driver.FindUserByID(filter.TargetUID)

    if err != nil {
        return err
    }

    if err := authorization.Authorize(
        authorization.Action.ChangeRoles,
        authorization.Resource.User,
        filter.RequesterRoles,
    ); err != nil {
        return err
    }

    audit := newAudit(updatedOperation, filter, user)

    query := newQuery(
        `UPDATE "user" SET roles = $1
        WHERE id = $2;`,
        newRoles, filter.TargetUID,
    )

    return cache.Client.DeleteOnError(
        execWithAudit(&audit, query),
        cache.KeyBase[cache.UserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserRolesById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    )
}

func (_ *repository) Activate(tk string) *Error.Status {
    t, err := token.ParseSingedToken(tk, config.Secret.ActivationTokenPublicKey)
    if err != nil {
        return err
    }

    payload, err := UserMapper.PayloadFromClaims(t.Claims.(jwt.MapClaims))
    if err != nil {
        return err
    }

    user, err := driver.FindUserByID(payload.ID)
    if err != nil {
        return err
    }
    if user.IsActive() {
        return Error.NewStatusError(
            "User already active",
            http.StatusConflict,
        )
    }

    filter :=  &UserDTO.Filter{
        TargetUID: user.ID,
        RequesterUID: user.ID,
        RequesterRoles: user.Roles,
    }

    audit := newAudit(updatedOperation, filter, user)

    for i, role := range user.Roles {
        if role == "unconfirmed_user" {
            user.Roles[i] = "user"
            break
        }
    }

    tx := newTransaction(
        newQuery(
            `UPDATE "user" SET roles = $1
             WHERE login = $2;`,
             user.Roles, user.Login,
        ),
        newAuditQuery(&audit),
    )
    if err := tx.Exec(); err != nil {
        return err
    }

    if err := cache.Client.Delete(
        cache.KeyBase[cache.UserById] + filter.TargetUID,
        cache.KeyBase[cache.AnyUserById] + filter.TargetUID,
        cache.KeyBase[cache.UserByLogin] + user.Login,
        cache.KeyBase[cache.AnyUserByLogin] + user.Login,
    ); err != nil {
        return Error.StatusInternalError
    }

    return nil
}

