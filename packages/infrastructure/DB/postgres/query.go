package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sentinel/packages/common/config"
	pbencoding "sentinel/packages/common/encoding/protobuf"
	Error "sentinel/packages/common/errors"
	UserDTO "sentinel/packages/core/user/DTO"
	"sentinel/packages/infrastructure/cache"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type executable interface {
    Exec() *Error.Status
}

type query struct {
    sql string
    args []any
}

func newQuery(sql string, args ...any) *query {
    return &query{sql, args}
}

func (q *query) toStatusError(err error) *Error.Status {
    defer dbLogger.Debug("Failed query: " + q.sql, nil)

    if err == context.DeadlineExceeded {
        dbLogger.Error("Query failed", "Query timeout", nil)
        return Error.StatusTimeout
    }

    dbLogger.Error("Query failed", err.Error(), nil)
    return Error.StatusInternalError
}

type queryMode int

const (
	execMode queryMode = iota
	rowsMode
	rowMode
)

// Executes given SQL. If returnRow is true then returns resulting row and error,
// otherwise returns nil and error.
// Also substitutes query args (see pgx docs for details).
func(q *query) runSQL(conType connectionType, mode queryMode) (pgx.Row, pgx.Rows, *Error.Status) {
    con, err := driver.getConnection(conType)

    if err != nil {
        return nil, nil, err
    }

    defer con.Release()

    ctx, cancel := defaultTimeoutContext()

    defer cancel()

	if config.Debug.Enabled && config.Debug.LogDbQueries {
		args := make([]string, len(q.args))

		for i, arg := range q.args {
			switch a := arg.(type) {
			case string:
				args[i] = a
			case []string:
				args[i] = strings.Join(a, ", ")
			case int:
				args[i] = strconv.FormatInt(int64(a), 10)
			case int64:
				args[i] = strconv.FormatInt(a, 10)
			case int32:
				args[i] = strconv.FormatInt(int64(a), 10)
			}
		}

		dbLogger.Debug("Running query:\n" + q.sql + "\nQuery args: " + strings.Join(args, "; "), nil)
	}

	switch mode {
	case execMode:
		if _, e := con.Exec(ctx, q.sql, q.args...); e != nil {
			return nil, nil, q.toStatusError(e)
		}
		return nil, nil, nil
	case rowMode:
		return con.QueryRow(ctx, q.sql, q.args...), nil, nil
	case rowsMode:
		r, e := con.Query(ctx, q.sql, q.args...)
		if e != nil {
			return nil, nil, q.toStatusError(e)
		}
		return nil, r, nil
	}

	dbLogger.Panic(
		"Failed to execute SQL",
		fmt.Sprintf("Unexpected query mode: %v", mode),
		nil,
	)
	return nil, nil, nil
}

func (q *query) Rows(conType connectionType) (pgx.Rows, *Error.Status) {
	_, rows, err := q.runSQL(conType, rowsMode)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// Scans a row into the given destinations.
// All dests must be pointers.
// By default, dests are not validated,
// but it can be added by setting env variable DEBUG_SAFE_DB_SCANS to true.
// (works only if app launched in debug mode)
type scanRowFunc = func(dests ...any) *Error.Status

// Wrapper for '*pgxpool.Con.QueryRow'
func (q *query) Row(conType connectionType) (scanRowFunc, *Error.Status) {
    row, _, err := q.runSQL(conType, rowMode)
    if err != nil {
        return nil, err
    }

    return func (dests ...any) *Error.Status {
        if config.Debug.Enabled && config.Debug.SafeDatabaseScans {
            for _, dest := range dests {
                typeof := reflect.TypeOf(dest)

                if typeof.Kind() != reflect.Ptr {
                    dbLogger.Panic(
                        "Query scan failed",
                        "Destination for scanning must be a pointer, but got '"+typeof.String()+"'",
						nil,
                    )
                }
            }
        }

        if e := row.Scan(dests...); e != nil {
            if errors.Is(e, pgx.ErrNoRows) {
                return Error.StatusNotFound
            }

            return q.toStatusError(e)
        }

        return nil
    }, nil
}

// Wrapper for '*pgxpool.Con.Exec'
func (q *query) Exec(conType connectionType) (*Error.Status) {
    _, _, err := q.runSQL(conType, execMode)
    return err
}

// TODO add cache
func collect[T UserDTO.Any](
	conType connectionType,
	q *query,
	collectFunc func(pgx.CollectableRow) (T, error),
) ([]T, *Error.Status) {
    rows, err := q.Rows(conType)
    if err != nil {
        return nil, err
    }

	dtos, e := pgx.CollectRows(rows, collectFunc)

    if e != nil {
		dbLogger.Error("Failed to collect rows", e.Error(), nil)
        return nil, q.toStatusError(e)
    }
	if len(dtos) == 0 {
		return nil, Error.StatusNotFound
	}

    return dtos, nil
}

// TODO add cache
func (q *query) CollectBasicUserDTO(conType connectionType) ([]*UserDTO.Basic, *Error.Status) {
	return collect(conType, q, func (row pgx.CollectableRow) (*UserDTO.Basic, error) {
		dto := new(UserDTO.Basic)

		var deletedAt sql.NullTime

		if err := row.Scan(
			&dto.ID,
			&dto.Login,
			&dto.Password,
			&dto.Roles,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if deletedAt.Valid {
			dto.DeletedAt = deletedAt.Time
		}

		return dto, nil
	})
}

// TODO add cache
func (q *query) CollectPublicUserDTO(conType connectionType) ([]*UserDTO.Public, *Error.Status) {
	return collect(conType, q, func (row pgx.CollectableRow) (*UserDTO.Public, error) {
		dto := new(UserDTO.Public)

		var deletedAt sql.NullTime

		if err := row.Scan(
			&dto.ID,
			&dto.Login,
			&dto.Roles,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if deletedAt.Valid {
			dto.DeletedAt = &deletedAt.Time
		}

		return dto, nil
	})
}

// Works same as queryRow, but also creates and returns
// UserDTO.Basic after scanning resulting row into it.
func (q *query) BasicUserDTO(conType connectionType, cacheKey string) (*UserDTO.Basic, *Error.Status) {
    if cached, hit := cache.Client.Get(cacheKey); hit {
		r, err := pbencoding.UnmarshallBasicUserDTO([]byte(cached))

        // r, err := json.DecodeString[UserDTO.Basic](cached)

        if err == nil {
            return r, nil
        }

        // if json decoding failed thats mean more likely it was invalid,
        // so deleting it from cache to prevent futher cache errors.
        // if it keep repeating even after this, then smth really went wrong.
        if e := cache.Client.Delete(cacheKey); e != nil {
            return nil, e
        }
    }

    scan, err := q.Row(conType)

    if err != nil {
        return nil, err
    }

    dto := new(UserDTO.Basic)

    var deletedAt sql.NullTime

    err = scan(
        &dto.ID,
        &dto.Login,
        &dto.Password,
        &dto.Roles,
        &deletedAt,
    )

    if err != nil {
        return nil, err
    }

	if deletedAt.Valid {
		dto.DeletedAt = deletedAt.Time
	}

	cached, e := pbencoding.MarshallBasicUserDTO(dto)
	if e != nil {
		dbLogger.Error(
			"Failed to encode basic user DTO",
			e.Error(),
			nil,
		)
	} else {
    	cache.Client.Set(cacheKey, cached)
	}

    return dto, nil
}

