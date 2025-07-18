package query

import (
	"context"
	Error "sentinel/packages/common/errors"
	"sentinel/packages/common/logger"
)

var queryLogger = logger.NewSource("QUERY", logger.Default)

type Query struct {
    SQL 		string
    Args 		[]any
}

func New(sql string, args ...any) *Query {
	return &Query{
		SQL: sql,
		Args: args,
	}
}

func (q *Query) ConvertError(err error) *Error.Status {
    defer queryLogger.Debug("Failed query: " + q.SQL, nil)

    if err == context.DeadlineExceeded {
        queryLogger.Error("Query failed", "Query timeout", nil)
        return Error.StatusTimeout
    }

    queryLogger.Error("Query failed", err.Error(), nil)
    return Error.StatusInternalError
}

