package localmanager

import (
	"flag"
	"os"
	"reflect"

	localcontroller "configmap-controller/controller"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsServer "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	logTimeLayout    = "2006-01-02-15:04:05.000-MST"
	controllerLeader = "control-plane.alpha.kubernetes.io/leader"
)

func init() {
	log.SetLogger(zap.New())
}

func RunManager() error {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var leaderNs string

	flag.StringVar(&leaderNs, "leader-namespace", "default", "leader namespace")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.TimeEncoderOfLayout(logTimeLayout),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	mgrLog := log.Log.WithName("kube-controller")

	// Setup a Manager
	mgrLog.Info("setting up manager")
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Metrics: metricsServer.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: leaderNs,
		LeaderElectionID:        "241011712.some-controll.cn",
	})
	if err != nil {
		mgrLog.Error(err, "err to set up confgigmap manager")
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		mgrLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		mgrLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	c, err := controller.New("configmap-controller", mgr, controller.Options{
		Reconciler:   &localcontroller.ReconcileConfig{Client: mgr.GetClient()},
		RecoverPanic: func() *bool { t := true; return &t }(),
	})
	if err != nil {
		mgrLog.Error(err, "err to set up configmap controller")
		return err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &corev1.ConfigMap{}, &handler.TypedEnqueueRequestForObject[*corev1.ConfigMap]{},
		predicate.TypedFuncs[*corev1.ConfigMap]{
			CreateFunc: func(tce event.TypedCreateEvent[*corev1.ConfigMap]) bool {
				return false
			},
			UpdateFunc: func(tue event.TypedUpdateEvent[*corev1.ConfigMap]) bool {
				// ignore leader configmap
				if _, ok := tue.ObjectNew.GetAnnotations()[controllerLeader]; ok {
					return false
				}
				// only data changed and have special labels will trigger reconcile
				if v, ok := tue.ObjectNew.GetLabels()[localcontroller.ConfigRetartKey]; !(ok && v == localcontroller.ConfigRetartValue) {
					return false
				}

				return !reflect.DeepEqual(tue.ObjectOld.Data, tue.ObjectNew.Data)
			},
			DeleteFunc:  func(tde event.TypedDeleteEvent[*corev1.ConfigMap]) bool { return false },
			GenericFunc: func(tge event.TypedGenericEvent[*corev1.ConfigMap]) bool { return false },
		},
	))
	if err != nil {
		mgrLog.Error(err, "unable to watch configmap")
		return err
	}

	mgrLog.Info("starting configmap manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		mgrLog.Error(err, "unable to run configmap manager")
		return err
	}
	return nil

}
