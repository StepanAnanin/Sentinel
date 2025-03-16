package postgres

import (
	UserDTO "sentinel/packages/core/user/DTO"
	Error "sentinel/packages/errors"
	"sentinel/packages/infrastructure/auth/authorization"
	"sentinel/packages/infrastructure/cache"
	"strings"
)

type seeker struct {
    //
}

// TODO now value can be only a string, rework that.
//      (make it a function with some generic instead of seeker's method?)
func (s *seeker) findUserBy(
    conditionProperty userProperty,
    conditionPropertyValue string,
    state userState,
    cacheKey string,
) (*UserDTO.Indexed, *Error.Status) {
    user, err := queryDTO(
        cacheKey,
        `SELECT id, login, password, roles, deletedAt
         FROM "user"
         WHERE ` + string(conditionProperty) + ` = $1;`,
        conditionPropertyValue,
    )

    if err != nil {
        return nil, err
    }

    if state == deletedUserState && user.DeletedAt == 0 {
        return nil, Error.StatusUserNotFound
    }

    if state == notDeletedUserState && user.DeletedAt != 0 {
        return nil, Error.StatusUserNotFound
    }

    return user, nil
}

func (s *seeker) FindAnyUserByID(id string) (*UserDTO.Indexed, *Error.Status) {
    return s.findUserBy(
        idProperty,
        id,
        anyUserState,
        cache.KeyBase[cache.AnyUserById] + id,
    )
}

func (s *seeker) FindUserByID(id string) (*UserDTO.Indexed, *Error.Status) {
    return s.findUserBy(
        idProperty,
        id,
        notDeletedUserState,
        cache.KeyBase[cache.UserById] + id,
    )
}

func (s *seeker) FindSoftDeletedUserByID(id string) (*UserDTO.Indexed, *Error.Status) {
    return s.findUserBy(
        idProperty,
        id,
        deletedUserState,
        cache.KeyBase[cache.DeletedUserById] + id,
    )
}

func (s *seeker) FindAnyUserByLogin(login string) (*UserDTO.Indexed, *Error.Status) {
    return s.findUserBy(
        loginProperty,
        login,
        anyUserState,
        cache.KeyBase[cache.AnyUserByLogin] + login,
    )
}

func (s *seeker) FindUserByLogin(login string) (*UserDTO.Indexed, *Error.Status) {
    return s.findUserBy(
        loginProperty,
        login,
        notDeletedUserState,
        cache.KeyBase[cache.UserByLogin] + login,
    )
}

func (s *seeker) IsLoginExists(login string) (bool, *Error.Status) {
    cacheKey := cache.KeyBase[cache.UserByLogin] + login

    _, err := s.findUserBy(loginProperty, login, notDeletedUserState, cacheKey)

    if err != nil {
        if err == Error.StatusUserNotFound {
            cache.Client.Set(cacheKey, false)
            return false, nil
        }

        return false, err
    }

    return true, nil
}

func (_ *seeker) GetRoles(filter *UserDTO.Filter) ([]string, *Error.Status) {
    if err := authorization.Authorize(
        authorization.Action.GetRoles,
        authorization.Resource.User,
        filter.RequesterRoles,
    ); err != nil {
        return nil, err
    }

    if rawRoles, hit := cache.Client.Get(cache.KeyBase[cache.UserRolesById] + filter.TargetUID); hit {
        return strings.Split(rawRoles, ","), nil
    }

    sql := `SELECT roles FROM "user" WHERE id = $1 AND deletedAt = 0;`

    scan, err := queryRow(sql, filter.TargetUID)

    if err != nil {
        return nil, err
    }

    roles := []string{}

    if e := scan(false, &roles); e != nil {
        return nil, e
    }

    cache.Client.Set(cache.KeyBase[cache.UserRolesById] + filter.TargetUID, strings.Join(roles, ","))

    return roles, nil
}

