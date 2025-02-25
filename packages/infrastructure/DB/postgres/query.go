package postgres

import (
	"context"
	"errors"
	UserDTO "sentinel/packages/core/user/DTO"
	Error "sentinel/packages/errors"

	"github.com/jackc/pgx/v5"
)

// Executes given sql.
// Substitutes given args (see pgx docs for details).
func evalSQL(sql string, args ...any) (pgx.Row, *Error.Status) {
    con, err := driver.getConnection()

    defer con.Release()

    if err != nil {
        return nil, err
    }

    ctx, cancel := defaultTimeoutContext()

    defer cancel()

    return con.QueryRow(ctx, sql, args...), nil
}

func dtoFromQuery(sql string, args ...any) (*UserDTO.Indexed, *Error.Status) {
    dto := new(UserDTO.Indexed)

    row, err := evalSQL(sql, args)

    if err != nil {
        if err == context.DeadlineExceeded {
            return nil, Error.StatusTimeout
        }

        return nil, err
    }

    e := row.Scan(&dto.ID, &dto.Login, &dto.Password, &dto.Roles, &dto.DeletedAt)

    if e != nil {
        if errors.Is(e, pgx.ErrNoRows) {
            return nil, Error.StatusUserNotFound
        }

        println(e.Error())

        return nil, Error.StatusInternalError
    }

    return dto, nil
}


