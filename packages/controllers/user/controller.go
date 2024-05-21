package user

import (
	"log"
	"net/http"
	ExternalError "sentinel/packages/error"
	"sentinel/packages/json"
	"sentinel/packages/models/token"
	user "sentinel/packages/models/user"
	"sentinel/packages/net"

	"go.mongodb.org/mongo-driver/mongo"
)

type Controller struct {
	user  *user.Model
	token *token.Model
}

func New(dbClient *mongo.Client) *Controller {
	return &Controller{
		user:  user.New(dbClient),
		token: token.New(dbClient),
	}
}

func (c Controller) Create(w http.ResponseWriter, req *http.Request) {
	if ok := net.Request.Preprocessing(w, req, http.MethodPost); !ok {
		return
	}

	body, ok := json.Decode[net.AuthRequestBody](req.Body, w)

	if !ok {
		if err := net.Response.InternalServerError(w); err != nil {
			panic(err)
		}
	}

	_, err := c.user.Create(body.Email, body.Password)

	if err != nil {
		ok, e := ExternalError.Is(err)

		if !ok {
			net.Response.Message("Не удалось создать пользователя: Внутреняя ошибка сервера.", http.StatusInternalServerError, w)

			log.Fatalln(err)
		}

		net.Response.Message(e.Message, e.Status, w)

		net.Request.PrintError("Failed to create new user: "+e.Message, e.Status, req)

		return
	}

	if err := net.Response.OK(w); err != nil {
		panic(err)
	}

	net.Request.Print("New user created, email: "+body.Email, req)
}

func (c Controller) UNSAFE_ChangeEmail(w http.ResponseWriter, req *http.Request) {
	net.Response.InternalServerError(w)

	log.Fatalln("[ CRITICAL ERROR] Method not implemented")
}

func (c Controller) UNSAFE_ChangePassword(w http.ResponseWriter, req *http.Request) {
	net.Response.InternalServerError(w)

	log.Fatalln("[ CRITICAL ERROR] Method not implemented")
}

func (c Controller) UNSAFE_ChangeRole(w http.ResponseWriter, req *http.Request) {
	net.Response.InternalServerError(w)

	log.Fatalln("[ CRITICAL ERROR] Method not implemented")
}

func (c Controller) UNSAFE_SoftDelete(w http.ResponseWriter, req *http.Request) {
	net.Response.InternalServerError(w)

	log.Fatalln("[ CRITICAL ERROR] Method not implemented")
}

func (c Controller) UNSAFE_HardDelete(w http.ResponseWriter, req *http.Request) {
	net.Response.InternalServerError(w)

	log.Fatalln("[ CRITICAL ERROR] Method not implemented")
}
