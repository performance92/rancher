package common

// XXX TODO AK -- marker of code modified for ext token support

import (
	"context"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/accessor"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

const (
	// UserAttributePrincipalID is a key in the ExtraByProvider field of the UserAttribute that holds principal IDs for a partucular provider.
	UserAttributePrincipalID = "principalid"
	// UserAttributeUserName is a key in the ExtraByProvider field of the UserAttribute that holds usernames for a partucular provider.
	UserAttributeUserName = "username"
	// UserPrincipalType is the user principal type across all providers.
	UserPrincipalType = "user"
	// GroupPrincipalType is the group principal type  across all providers.
	GroupPrincipalType = "group"
)

type AuthProvider interface {
	GetName() string
	AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error)
	SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]v3.Principal, error)
	GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error)
	CustomizeSchema(schema *types.Schema)
	TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error)
	RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error)
	CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error)
	GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string
	IsDisabledProvider() (bool, error)
}
