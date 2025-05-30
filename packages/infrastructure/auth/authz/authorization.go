package authz

import (
	"net/http"
	"sentinel/packages/common/config"
	Error "sentinel/packages/common/errors"
	"sentinel/packages/common/logger"
	"strings"

	rbac "github.com/StepanAnanin/SentinelRBAC"
)

var authzLogger = logger.NewSource("AUTHZ", logger.Default)

type resource struct {
	User  *rbac.Resource
	Cache *rbac.Resource
}

var Resource resource
var Host *rbac.Host
var schema *rbac.Schema

func Init() {
	authzLogger.Info("Loading Host configuration...")

	h, e := rbac.LoadHost("RBAC.json")
	if e != nil {
        authzLogger.Fatal("Failed to load Host configuration", e.Error())
	}

	authzLogger.Info("Loading Host configuration: OK")
	authzLogger.Info("Getting schema for this service...")

    Host = &h

	s, err := Host.GetSchema(config.App.ServiceID)
	if err != nil {
		authzLogger.Fatal("Failed to get schema for this service", err.Error())
	}

	authzLogger.Info("Getting schema for this service: OK")
	authzLogger.Info("Initializing resources...")

    schema = s

    Resource = resource{
        User: rbac.NewResource("user", schema.Roles),

        Cache: rbac.NewResource("cache", (func() []rbac.Role {
            roles := make([]rbac.Role, len(schema.Roles))

            for i, role := range schema.Roles {
                // Only admins can interact with cache
                if role.Name == "admin" {
                    roles[i] = role
                } else {
                    roles[i] = rbac.NewRole(role.Name, 0)
                }
            }

            return roles
        })()),
    }

	authzLogger.Info("Initializing resources: OK")
}

var user = rbac.NewEntity("user")

var insufficientPermissions = Error.NewStatusError(
    "Недостаточно прав для выполнения данной операции",
    http.StatusForbidden,
)

// TODO is there any point in this function? why just don't use resource.Authorize(...)?

// Checks if user with specified roles can perform action on given resource.
// Returns *Error.Status if user has insufficient permissions or smth is missconfigured, otherwise returns nil.
//
// This method authorize operations only in THIS service!
// Operations on other services must be authorized by themselves!
func Authorize(action rbac.Action, resource *rbac.Resource, userRoles []string) *Error.Status {
	authzLogger.Debug("Authorizing "+action.String()+"...")

	err := resource.Authorize(action, userRoles)

    if err != nil {
		authzLogger.Debug("Authorization of "+action.String()+" failed: " + err.Error())

        if err == rbac.InsufficientPermissions {
            return insufficientPermissions
        }

		authzLogger.Error(
			"Unexpected error occured on authorizing "+action.String()+" (on resource "+resource.Name+") for " + strings.Join(userRoles, ","),
			err.Error(),
		)

        // if err is not nil and not rbac.InsufficientPermissions that means
        // resource permissions wasn't defined for some one of given roles
        return Error.NewStatusError(
            err.Error(),
            http.StatusInternalServerError,
        )
    }

	authzLogger.Debug("Authorizing "+action.String()+": OK")

    return nil
}

