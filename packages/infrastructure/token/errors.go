package token

import (
	"net/http"
	Error "sentinel/packages/common/errors"
)

// TODO handle all this errors

var TokenMalformed = Error.NewStatusError(
    "Token is malformed or has invalid format",
    // According to RFC 7235 (https://datatracker.ietf.org/doc/html/rfc7235#section-3.1)
    // 401 response status code indicates that the request lacks VALID authentication credentials,
    // no matter if token was invalid, missing or auth creditinals is invalid.
    http.StatusUnauthorized,
)

var TokenExpired = Error.NewStatusError(
    "Token expired",
    http.StatusUnauthorized,
)

var InvalidToken = Error.NewStatusError(
    "Invalid Token",
    http.StatusBadRequest,
)

var TokenModified = Error.NewStatusError(
    "Invalid Token (and you know that)",
    http.StatusBadRequest,
)

func IsTokenError(err *Error.Status) bool {
    return err == TokenMalformed ||
           err == TokenExpired ||
           err == TokenModified ||
           err == InvalidToken
}

