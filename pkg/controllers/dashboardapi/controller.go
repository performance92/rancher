package dashboardapi

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/feature"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Add(ctx context.Context, wrangler *wrangler.Context) error {
	if err := addClusterRepos(ctx, wrangler); err != nil {
		return err
	}

	return nil
}

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	feature.Register(ctx, wrangler.Mgmt.Feature())
	helm.RegisterReposForFollowers(ctx, wrangler.Core.Secret().Cache(), wrangler.Catalog.ClusterRepo())
	return settings.Register(wrangler.Mgmt.Setting())
}
