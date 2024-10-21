package localcontroller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ReconcileConfig struct {
	Client client.Client
}

var (
	resourceType = "ConfigMap"

	ConfigRetartKey   = "configrestart/deployment"
	ConfigRetartValue = "enable"

	configMapControllerRestartAnnotation = "configmap-controller/restart"
	configMapControllerFieldManager      = "configmap-controller"
)

var _ reconcile.Reconciler = &ReconcileConfig{}

func (r *ReconcileConfig) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("start reconcile", "rescoures type", resourceType, "name", request.Name, "namespace", request.Namespace)

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, request.NamespacedName, cm)
	if k8s_errors.IsNotFound(err) {
		log.Error(nil, "resources not found", "rescoures type", resourceType, "name", request.Name, "namespace", request.Namespace)
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get %s name=%s ns=%s, err=%v", resourceType, request.Name, request.Namespace, err)
	}

	// main logical
	if v, ok := cm.Labels[ConfigRetartKey]; ok && v == ConfigRetartValue {
		// restart deployment
		err := restartDeploymentWithConfigMap(ctx, r.Client, cm.Name, cm.Namespace)
		if err != nil {
			log.Error(err, "restart deployment error, not requened", "configMap name", cm.Name, "deployment", cm.Name, "namespace", cm.Namespace)
			return reconcile.Result{Requeue: false}, err
		}
		log.Info("restart deployment success", "configMap name", cm.Name, "deployment", cm.Name, "namespace", cm.Namespace)
	}
	return reconcile.Result{}, nil
}

func restartDeployment(ctx context.Context, c client.Client, deploymentName, ns string) error {

	var deploy appsv1.Deployment

	err := c.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: ns}, &deploy)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			slog.Info("Deployment not found, skip restart", "deployment", deploymentName, "namespace", ns)
			return nil
		}
		return err
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations[configMapControllerRestartAnnotation] = time.Now().Format(time.RFC3339)

	err = c.Update(ctx, &deploy, &client.UpdateOptions{FieldManager: configMapControllerFieldManager})
	if err != nil {
		return err
	}

	return nil
}
func checkDeploymentHasConfigMap(configMapName string, deploy *appsv1.Deployment) bool {

	for _, volume := range deploy.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name == configMapName {
			return true
		}
	}

	return false
}

func restartDeploymentWithConfigMap(ctx context.Context, c client.Client, configName, ns string) error {

	var deploys appsv1.DeploymentList
	err := c.List(ctx, &deploys, &client.ListOptions{Namespace: ns})
	if err != nil {
		return err
	}

	for _, deploy := range deploys.Items {
		if checkDeploymentHasConfigMap(configName, &deploy) {
			err := restartDeployment(ctx, c, deploy.Name, ns)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
