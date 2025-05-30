package token

import (
	"net/http"
	Error "sentinel/packages/common/errors"

	"github.com/golang-jwt/jwt/v5"
)

var invalidTokenPayload = Error.NewStatusError(
    "Invalid token payload",
    http.StatusBadRequest,
)

func VerifyClaims(claims jwt.MapClaims) *Error.Status {
	if claims[ServiceIdClaimsKey] == nil {
		return invalidTokenPayload
    }
	if claims[ExpiresAtClaimsKey] == nil {
		return invalidTokenPayload
    }
	if claims[IssuedAtClaimsKey] == nil {
		return invalidTokenPayload
    }
	if claims[UserIdClaimsKey] == nil {
		return invalidTokenPayload
    }
	if claims[UserLoginClaimsKey] == nil {
		return invalidTokenPayload
    }
	if claims[UserRolesClaimsKey] == nil {
		return invalidTokenPayload
    }
	return nil
}

