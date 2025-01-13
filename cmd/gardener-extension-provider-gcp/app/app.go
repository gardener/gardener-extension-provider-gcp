// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	heartbeatcmd "github.com/gardener/gardener/extensions/pkg/controller/heartbeat/cmd"
	"github.com/gardener/gardener/extensions/pkg/util"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/secrets"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/internal/handler/tokenmeta"
	gcpinstall "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	gcpcmd "github.com/gardener/gardener-extension-provider-gcp/pkg/cmd"
	gcpbackupbucket "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupbucket"
	gcpbackupentry "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupentry"
	gcpbastion "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/bastion"
	gcpcontrolplane "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/controlplane"
	gcpdnsrecord "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/healthcheck"
	gcpinfrastructure "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	gcpworker "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpseedprovider "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/seedprovider"
)

// NewControllerManagerCommand creates a new command for running a GCP provider controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		generalOpts = &controllercmd.GeneralOptions{}
		restOpts    = &controllercmd.RESTOptions{}
		mgrOpts     = &controllercmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        controllercmd.LeaderElectionNameID(gcp.Name),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:       443,
			WebhookCertDir:          "/tmp/gardener-extensions-cert",
			MetricsBindAddress:      ":8080",
			HealthBindAddress:       ":8081",
		}
		configFileOpts = &gcpcmd.ConfigOptions{}

		// options for the backupbucket controller
		backupBucketCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the backupentry controller
		backupEntryCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the bastion controller
		bastionCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the health care controller
		healthCheckCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the heartbeat controller
		heartbeatCtrlOpts = &heartbeatcmd.Options{
			ExtensionName:        gcp.Name,
			RenewIntervalSeconds: 30,
			Namespace:            os.Getenv("LEADER_ELECTION_NAMESPACE"),
		}

		// options for the controlplane controller
		controlPlaneCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the infrastructure controller
		infraCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &controllercmd.ReconcilerOptions{}

		// options for the worker controller
		workerCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the webhook server
		webhookServerOptions = &webhookcmd.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		controllerSwitches = gcpcmd.ControllerSwitchOptions()
		webhookSwitches    = gcpcmd.WebhookSwitchOptions()
		webhookOptions     = webhookcmd.NewAddToManagerOptions(
			gcp.Name,
			genericactuator.ShootWebhooksResourceName,
			genericactuator.ShootWebhookNamespaceSelector(gcp.Type),
			webhookServerOptions,
			webhookSwitches,
		)

		aggOption = controllercmd.NewOptionAggregator(
			generalOpts,
			restOpts,
			mgrOpts,
			controllercmd.PrefixOption("backupbucket-", backupBucketCtrlOpts),
			controllercmd.PrefixOption("backupentry-", backupEntryCtrlOpts),
			controllercmd.PrefixOption("bastion-", bastionCtrlOpts),
			controllercmd.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			controllercmd.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			controllercmd.PrefixOption("infrastructure-", infraCtrlOpts),
			controllercmd.PrefixOption("worker-", workerCtrlOpts),
			controllercmd.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			controllercmd.PrefixOption("heartbeat-", heartbeatCtrlOpts),
			configFileOpts,
			controllerSwitches,
			reconcileOpts,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", gcp.Name),

		RunE: func(_ *cobra.Command, _ []string) error {
			verflag.PrintAndExitIfRequested()

			if err := aggOption.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			if err := heartbeatCtrlOpts.Validate(); err != nil {
				return err
			}

			if err := features.ExtensionFeatureGate.SetFromMap(configFileOpts.Completed().Config.FeatureGates); err != nil {
				return err
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}

			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := gcpinstall.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := druidv1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := autoscalingv1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := machinev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := monitoringv1.AddToScheme(mgr.GetScheme()); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}

			// add common meta types to schema for controller-runtime to use v1.ListOptions
			metav1.AddToGroupVersion(scheme, machinev1alpha1.SchemeGroupVersion)

			log := mgr.GetLogger()
			log.Info("Getting rest config for garden")
			gardenRESTConfig, err := kubernetes.RESTConfigFromKubeconfigFile(os.Getenv("GARDEN_KUBECONFIG"), kubernetes.AuthTokenFile)
			if err != nil {
				return err
			}

			log.Info("Setting up cluster object for garden")
			gardenCluster, err := cluster.New(gardenRESTConfig, func(opts *cluster.Options) {
				opts.Scheme = kubernetes.GardenScheme
				opts.Logger = log
			})
			if err != nil {
				return fmt.Errorf("failed creating garden cluster object: %w", err)
			}

			log.Info("Adding garden cluster to manager")
			if err := mgr.Add(gardenCluster); err != nil {
				return fmt.Errorf("failed adding garden cluster to manager: %w", err)
			}

			log.Info("Adding controllers to manager")
			configFileOpts.Completed().ApplyETCDStorage(&gcpseedprovider.DefaultAddOptions.ETCDStorage)
			configFileOpts.Completed().ApplyHealthCheckConfig(&healthcheck.DefaultAddOptions.HealthCheckConfig)
			healthCheckCtrlOpts.Completed().Apply(&healthcheck.DefaultAddOptions.Controller)
			heartbeatCtrlOpts.Completed().Apply(&heartbeat.DefaultAddOptions)
			backupBucketCtrlOpts.Completed().Apply(&gcpbackupbucket.DefaultAddOptions.Controller)
			backupEntryCtrlOpts.Completed().Apply(&gcpbackupentry.DefaultAddOptions.Controller)
			bastionCtrlOpts.Completed().Apply(&gcpbastion.DefaultAddOptions.Controller)
			controlPlaneCtrlOpts.Completed().Apply(&gcpcontrolplane.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&gcpdnsrecord.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&gcpinfrastructure.DefaultAddOptions.Controller)
			reconcileOpts.Completed().Apply(&gcpinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation, &gcpinfrastructure.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&gcpcontrolplane.DefaultAddOptions.IgnoreOperationAnnotation, &gcpcontrolplane.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&gcpworker.DefaultAddOptions.IgnoreOperationAnnotation, &gcpworker.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&gcpbastion.DefaultAddOptions.IgnoreOperationAnnotation, &gcpbastion.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&gcpdnsrecord.DefaultAddOptions.IgnoreOperationAnnotation, &gcpdnsrecord.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&gcpbackupbucket.DefaultAddOptions.IgnoreOperationAnnotation, &gcpbackupbucket.DefaultAddOptions.ExtensionClass)
			workerCtrlOpts.Completed().Apply(&gcpworker.DefaultAddOptions.Controller)
			gcpworker.DefaultAddOptions.GardenCluster = gardenCluster

			// Generate certificates that will be used by the token metadata server
			// and the requesting http clients for communication. Since the communication is
			// contained to this process the certificates will not be persisted anywhere and will be kept in memory.
			ca, serverCert, clientCert, err := generateTokenMetaServerCerts()
			if err != nil {
				return fmt.Errorf("failed generating certificates for the token metadata server: %w", err)
			}

			systemCertPool, err := x509.SystemCertPool()
			if err != nil {
				return fmt.Errorf("could not retrieve the system certificate pool: %w", err)
			}
			systemCertPool.AppendCertsFromPEM(ca.CertificatePEM)
			tokenMetaClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs:      systemCertPool,
						Certificates: []tls.Certificate{clientCert},
					},
				},
			}
			// Inject the token metadata client and base URL to dependants that will require
			// to communicate with the token metadata server.
			tokenMetadataServerAddr := net.JoinHostPort("127.0.0.1", "50607")
			tokenMetadataServerBaseURL := "https://" + tokenMetadataServerAddr
			tokenMetadataURL := func(secretName, secretNamespace string) string {
				return tokenMetadataServerBaseURL + "/namespaces/" + secretNamespace + "/secrets/" + secretName + "/token"
			}
			gcpbastion.DefaultAddOptions.TokenMetadataClient = tokenMetaClient
			gcpdnsrecord.DefaultAddOptions.TokenMetadataClient = tokenMetaClient
			gcpinfrastructure.DefaultAddOptions.TokenMetadataClient = tokenMetaClient
			gcpbackupbucket.DefaultAddOptions.TokenMetadataClient = tokenMetaClient
			gcpbastion.DefaultAddOptions.TokenMetadataURL = tokenMetadataURL
			gcpdnsrecord.DefaultAddOptions.TokenMetadataURL = tokenMetadataURL
			gcpinfrastructure.DefaultAddOptions.TokenMetadataURL = tokenMetadataURL
			gcpbackupbucket.DefaultAddOptions.TokenMetadataURL = tokenMetadataURL

			shootWebhookConfig, err := webhookOptions.Completed().AddToManager(ctx, mgr, nil)
			if err != nil {
				return fmt.Errorf("could not add webhooks to manager: %w", err)
			}
			gcpcontrolplane.DefaultAddOptions.ShootWebhookConfig = shootWebhookConfig
			gcpcontrolplane.DefaultAddOptions.WebhookServerNamespace = webhookOptions.Server.Namespace

			if err := controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
				return fmt.Errorf("could not add controllers to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
				return fmt.Errorf("could not add readycheck for informers: %w", err)
			}

			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				return fmt.Errorf("could not add health check to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
				return fmt.Errorf("could not add ready check for webhook server to manager: %w", err)
			}

			mux := http.NewServeMux()
			tokenHandler := tokenmeta.New(mgr.GetClient(), mgr.GetLogger().WithName("token-meta-handler"))
			tokenHandler.RegisterRoutes(mux)

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(ca.CertificatePEM)
			server := &http.Server{
				Addr: tokenMetadataServerAddr,

				Handler: mux,
				TLSConfig: &tls.Config{
					Certificates: []tls.Certificate{serverCert},
					ClientCAs:    caCertPool,
					ClientAuth:   tls.RequireAndVerifyClientCert,
				},

				ReadTimeout:  time.Second * 15,
				WriteTimeout: time.Second * 15,
			}

			srvCh := make(chan error)
			serverCtx, cancelSrv := context.WithCancel(ctx)

			mgrCh := make(chan error)
			mgrCtx, cancelMgr := context.WithCancel(ctx)

			go func() {
				defer cancelSrv()
				mgrCh <- fmt.Errorf("error running manager: %w", mgr.Start(mgrCtx))
			}()

			go func() {
				defer cancelMgr()
				srvCh <- runServer(serverCtx, log, server)
			}()

			select {
			case err := <-mgrCh:
				return errors.Join(err, <-srvCh)
			case err := <-srvCh:
				return errors.Join(err, <-mgrCh)
			}
		},
	}

	verflag.AddFlags(cmd.Flags())
	aggOption.AddFlags(cmd.Flags())

	return cmd
}

// generateTokenMetaServerCerts generates and returns certificates for the token metadata server
// in the following order: CertificateAuthority, ServerCertificate, ClientCertificate
func generateTokenMetaServerCerts() (*secrets.Certificate, tls.Certificate, tls.Certificate, error) {
	oneYear := time.Hour * 24 * 365
	caConfig := secrets.CertificateSecretConfig{
		Name:       "workload-identity-token-metadata-server-ca",
		CommonName: "workload-identity-token-metadata-server-ca",
		CertType:   secrets.CACert,

		Validity: &oneYear,
	}

	ca, err := caConfig.GenerateCertificate()
	if err != nil {
		return nil, tls.Certificate{}, tls.Certificate{}, err
	}

	serverCertConfig := secrets.CertificateSecretConfig{
		Name:       "workload-identity-token-metadata-server",
		CommonName: "workload-identity-token-metadata-server",
		CertType:   secrets.ServerCert,

		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},

		SigningCA:                         ca,
		Validity:                          &oneYear,
		IncludeCACertificateInServerChain: true,
	}

	serverCert, err := serverCertConfig.GenerateCertificate()
	if err != nil {
		return nil, tls.Certificate{}, tls.Certificate{}, err
	}

	tlsServerCert, err := tls.X509KeyPair(serverCert.CertificatePEM, serverCert.PrivateKeyPEM)
	if err != nil {
		return nil, tls.Certificate{}, tls.Certificate{}, err
	}

	clientCertConfig := secrets.CertificateSecretConfig{
		Name:       "workload-identity-token-metadata-server-client",
		CommonName: "workload-identity-token-metadata-server-client",
		CertType:   secrets.ClientCert,

		SigningCA: ca,
		Validity:  &oneYear,
	}

	clientCert, err := clientCertConfig.GenerateCertificate()
	if err != nil {
		return nil, tls.Certificate{}, tls.Certificate{}, err
	}

	tlsClientCert, err := tls.X509KeyPair(clientCert.CertificatePEM, clientCert.PrivateKeyPEM)
	if err != nil {
		return nil, tls.Certificate{}, tls.Certificate{}, err
	}

	return ca, tlsServerCert, tlsClientCert, nil
}

// runServer starts the token metadata server. It returns if the context is canceled or the server cannot start initially.
func runServer(ctx context.Context, log logr.Logger, srv *http.Server) error {
	log = log.WithName("token-meta-server")
	errCh := make(chan error)
	go func(errCh chan<- error) {
		log.Info("Starts listening", "address", srv.Addr)
		defer close(errCh)
		if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("failed serving content: %w", err)
		} else {
			log.Info("Server stopped listening")
		}
	}(errCh)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("Shutting down")
		cancelCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := srv.Shutdown(cancelCtx)
		if err != nil {
			return fmt.Errorf("token metadata server failed graceful shutdown: %w", err)
		}
		log.Info("Shutdown successful")
		return nil
	}
}
