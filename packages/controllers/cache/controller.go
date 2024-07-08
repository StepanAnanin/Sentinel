package cachecontroller

import (
	"net/http"
	"sentinel/packages/cache"
	"sentinel/packages/json"
	"sentinel/packages/models/role"
	"sentinel/packages/models/token"
	"sentinel/packages/models/user"

	"github.com/StepanAnanin/weaver/http/response"
	"github.com/golang-jwt/jwt"
)

type Controller struct {
	user  *user.Model
	token *token.Model
}

func New(userModel *user.Model, tokenModel *token.Model) *Controller {
	return &Controller{
		user:  userModel,
		token: tokenModel,
	}
}

func (c *Controller) Drop(w http.ResponseWriter, req *http.Request) {
	res := response.New(w).Logged(req)

	body, ok := json.Decode[map[string]any](req.Body)

	if !ok {
		res.InternalServerError()
		return
	}

	accessToken, err := c.token.GetAccessToken(req)

	if err != nil {
		res.Message(err.Message, err.Status)
		return
	}

	filter, err := c.token.UserFilterFromClaims(body["UID"].(string), accessToken.Claims.(jwt.MapClaims))

	if err != nil {
		res.Message(err.Message, err.Status)
		return
	}

	if err = filter.RequesterRole.Verify(); err != nil {
		res.Message(err.Message, err.Status)
		return
	}

	if filter.RequesterRole != role.Administrator {
		res.Message("Недостаточно прав для выполнения данной операции", http.StatusForbidden)
		return
	}

	cache.Drop()

	res.OK()
}
