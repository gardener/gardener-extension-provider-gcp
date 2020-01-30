//go:generate mockgen -package=client -destination=mocks.go github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client Interface,FirewallsService,RoutesService,FirewallsListCall,RoutesListCall,FirewallsDeleteCall,RoutesDeleteCall

package client
