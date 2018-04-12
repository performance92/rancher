package preference

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/api/store/userscope"
	"github.com/rancher/types/apis/core/v1"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/client/management/v3"
)

const (
	NamespaceID = client.PreferenceFieldNamespaceId
)

func NewStore(nsClient v1.NamespaceInterface,
	rbClient rbacv1.RoleBindingInterface,
	clusterRoleLister rbacv1.ClusterRoleLister,
	crClient rbacv1.ClusterRoleInterface,
	store types.Store) types.Store {
	return userscope.NewStore(nsClient, rbClient, clusterRoleLister, crClient,
		&transform.Store{
			Store:       store,
			Transformer: transformer,
		})
}

func transformer(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opts *types.QueryOptions) (map[string]interface{}, error) {
	if data == nil {
		return nil, nil
	}

	ns := convert.ToString(data[NamespaceID])
	id := convert.ToString(data[types.ResourceFieldID])

	id = strings.TrimPrefix(id, ns+":")

	data[client.PreferenceFieldName] = id
	data[types.ResourceFieldID] = id

	return data, nil
}
