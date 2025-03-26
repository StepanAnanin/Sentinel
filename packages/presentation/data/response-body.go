package datamodel

type TokenResponseBody struct {
	Message     string `json:"message"`
	AccessToken string `json:"accessToken"`
    ExpiresIn   int    `json:"expiresIn"`
}

type MessageResponseBody struct {
	Message string `json:"message"`
}

type LoginExistanceResponseBody struct {
	Exists bool `json:"exists"`
}

type RolesResponseBody struct {
	Roles []string `json:"roles"`
}

