package email

import (
	_ "embed"
	"net/http"
	"sentinel/packages/common/config"
	Error "sentinel/packages/common/errors"
	"sentinel/packages/common/validation"
	"sentinel/packages/presentation/api"
	"strings"

	"gopkg.in/gomail.v2"
)

type UserActivationEmail struct {
    To string
    Token string
}

func NewUserActivationEmail(to string, token string) (*UserActivationEmail, *Error.Status) {
    if err := validation.Email(to); err != nil {
        if err == Error.InvalidValue {
            return nil, Error.NewStatusError(
                "Invlaid E-Mail format",
                http.StatusBadRequest,
            )
        }
        if err == Error.NoValue {
            return nil, Error.NewStatusError(
                "E-Mail is not specified",
                http.StatusBadRequest,
            )
        }
    }

    return &UserActivationEmail{ To: to, Token: token }, nil
}

//go:embed templates/activation-email.template.html
var activationEmailTemplate string

type activationEmailTemplateValues struct {
    ActivationURL string
}

var activationTokenPlaceholder = "{{token}}"

// Must be initialized via email.Init()
var ActivationEmailBody string

func createActivationEmailBody() {
    values := activationEmailTemplateValues{
        ActivationURL: api.GetBaseURL() + "/activate/" + activationTokenPlaceholder,
    }

    b, err := parseTemplate(activationEmailTemplate, values)
    if err != nil {
        panic(err.Error())
    }

    ActivationEmailBody = b
}

func (e *UserActivationEmail) Send() *Error.Status {
    email := gomail.NewMessage()

    email.SetHeader("From", config.Secret.MailerEmail)
    email.SetHeader("To", e.To)
    email.SetHeader("Subject", "Account activation")

    body := strings.ReplaceAll(ActivationEmailBody, activationTokenPlaceholder, e.Token)

    email.SetBody("text/html", body)

    if err := dialer.DialAndSend(email); err != nil {
        return Error.NewStatusError(
            "Failed to send email: " + err.Error(),
            http.StatusInternalServerError,
        )
    }

    return nil
}

