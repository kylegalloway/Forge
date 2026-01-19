package controller

import (
	"context"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/actions/common"
	"github.com/kylegalloway/forge/pkg/actions/uds"
	"github.com/kylegalloway/forge/pkg/actions/zarf"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
)

// ZarfBuildHandlerAdapter adapts zarf.BuildHandler to common.ActionHandler
type ZarfBuildHandlerAdapter struct {
	handler *zarf.BuildHandler
}

// NewZarfBuildHandlerAdapter creates a new adapter for zarf.BuildHandler
func NewZarfBuildHandlerAdapter(handler *zarf.BuildHandler) *ZarfBuildHandlerAdapter {
	return &ZarfBuildHandlerAdapter{handler: handler}
}

// Execute implements common.ActionHandler interface
func (a *ZarfBuildHandlerAdapter) Execute(ctx context.Context, resource *zarfv1alpha3.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return a.handler.Execute(ctx, resource, opts.ArtifactPVCName)
}

// ZarfPublishHandlerAdapter adapts zarf.PublishHandler to common.ActionHandler
type ZarfPublishHandlerAdapter struct {
	handler *zarf.PublishHandler
}

// NewZarfPublishHandlerAdapter creates a new adapter for zarf.PublishHandler
func NewZarfPublishHandlerAdapter(handler *zarf.PublishHandler) *ZarfPublishHandlerAdapter {
	return &ZarfPublishHandlerAdapter{handler: handler}
}

// Execute implements common.ActionHandler interface
func (a *ZarfPublishHandlerAdapter) Execute(ctx context.Context, resource *zarfv1alpha3.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return a.handler.Execute(ctx, resource, opts.ArtifactPath, opts.ArtifactPVCName)
}

// ZarfDeployHandlerAdapter adapts zarf.DeployHandler to common.ActionHandler
type ZarfDeployHandlerAdapter struct {
	handler *zarf.DeployHandler
}

// NewZarfDeployHandlerAdapter creates a new adapter for zarf.DeployHandler
func NewZarfDeployHandlerAdapter(handler *zarf.DeployHandler) *ZarfDeployHandlerAdapter {
	return &ZarfDeployHandlerAdapter{handler: handler}
}

// Execute implements common.ActionHandler interface
func (a *ZarfDeployHandlerAdapter) Execute(ctx context.Context, resource *zarfv1alpha3.ZarfPackageJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return a.handler.Execute(ctx, resource, opts.ArtifactPath, opts.ArtifactPVCName)
}

// UDSCreateHandlerAdapter adapts uds.CreateHandler to common.ActionHandler
type UDSCreateHandlerAdapter struct {
	handler *uds.CreateHandler
}

// NewUDSCreateHandlerAdapter creates a new adapter for uds.CreateHandler
func NewUDSCreateHandlerAdapter(handler *uds.CreateHandler) *UDSCreateHandlerAdapter {
	return &UDSCreateHandlerAdapter{handler: handler}
}

// Execute implements common.ActionHandler interface
func (a *UDSCreateHandlerAdapter) Execute(ctx context.Context, resource *udsv1alpha3.UDSBundleJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return a.handler.Execute(ctx, resource, opts.ArtifactPVCName)
}

// UDSPublishHandlerAdapter adapts uds.PublishHandler to common.ActionHandler
type UDSPublishHandlerAdapter struct {
	handler *uds.PublishHandler
}

// NewUDSPublishHandlerAdapter creates a new adapter for uds.PublishHandler
func NewUDSPublishHandlerAdapter(handler *uds.PublishHandler) *UDSPublishHandlerAdapter {
	return &UDSPublishHandlerAdapter{handler: handler}
}

// Execute implements common.ActionHandler interface
func (a *UDSPublishHandlerAdapter) Execute(ctx context.Context, resource *udsv1alpha3.UDSBundleJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return a.handler.Execute(ctx, resource, opts.ArtifactPath, opts.ArtifactPVCName)
}

// UDSDeployHandlerAdapter adapts uds.DeployHandler to common.ActionHandler
type UDSDeployHandlerAdapter struct {
	handler *uds.DeployHandler
}

// NewUDSDeployHandlerAdapter creates a new adapter for uds.DeployHandler
func NewUDSDeployHandlerAdapter(handler *uds.DeployHandler) *UDSDeployHandlerAdapter {
	return &UDSDeployHandlerAdapter{handler: handler}
}

// Execute implements common.ActionHandler interface
func (a *UDSDeployHandlerAdapter) Execute(ctx context.Context, resource *udsv1alpha3.UDSBundleJob, opts common.ExecuteOptions) (*actions.ActionResult, error) {
	return a.handler.Execute(ctx, resource, opts.ArtifactPath, opts.ArtifactPVCName)
}

// Compile-time assertions that adapters implement ActionHandler
var _ common.ActionHandler[*zarfv1alpha3.ZarfPackageJob] = (*ZarfBuildHandlerAdapter)(nil)
var _ common.ActionHandler[*zarfv1alpha3.ZarfPackageJob] = (*ZarfPublishHandlerAdapter)(nil)
var _ common.ActionHandler[*zarfv1alpha3.ZarfPackageJob] = (*ZarfDeployHandlerAdapter)(nil)
var _ common.ActionHandler[*udsv1alpha3.UDSBundleJob] = (*UDSCreateHandlerAdapter)(nil)
var _ common.ActionHandler[*udsv1alpha3.UDSBundleJob] = (*UDSPublishHandlerAdapter)(nil)
var _ common.ActionHandler[*udsv1alpha3.UDSBundleJob] = (*UDSDeployHandlerAdapter)(nil)
