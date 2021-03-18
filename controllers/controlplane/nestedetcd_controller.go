/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controlplane

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"text/template"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1alpha4 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha4"
)

// NestedEtcdReconciler reconciles a NestedEtcd object
type NestedEtcdReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	statefulsetOwnerKey   = ".metadata.controller"
	apiGVStr              = controlplanev1alpha4.GroupVersion.String()
	defaultStatefulSetURL = "https://raw.githubusercontent.com/kubernetes-sigs/" +
		"cluster-api-provider-nested/master/config/component-templates/" +
		"nestedetcd/nested-etcd-statefulset-template.yaml"
	defaultServiceURL = "https://raw.githubusercontent.com/kubernetes-sigs/" +
		"cluster-api-provider-nested/master/config/component-templates/" +
		"nestedetcd/nested-etcd-service-template.yaml"
)

//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=statefulset,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulset/status,verbs=get;update;patch

func (r *NestedEtcdReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nestedetcd", req.NamespacedName)
	log.Info("Reconciling NestedEtcd...")
	var netcd controlplanev1alpha4.NestedEtcd
	if err := r.Get(ctx, req.NamespacedName, &netcd); err != nil {
		return ctrl.Result{}, ctrlcli.IgnoreNotFound(err)
	}
	log.Info("creating NestedEtcd",
		"namespace", netcd.GetNamespace(),
		"name", netcd.GetName())

	// check if the ownerreference has been set by the NestedControlPlane controller.
	owner := getOwner(netcd)
	if owner == (metav1.OwnerReference{}) {
		// requeue the request if the owner NestedControlPlane has
		// not been set yet.
		log.Info("the owner has not been set yet, will retry later",
			"namespace", netcd.GetNamespace(),
			"name", netcd.GetName())
		return ctrl.Result{Requeue: true}, nil
	}

	var netcdSts appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      netcd.GetName(),
	}, &netcdSts); err != nil {
		if apierrors.IsNotFound(err) {
			// as the statefulset is not found, mark the NestedEtcd as unready
			if IsNetcdReady(netcd.Status) {
				netcd.Status.Phase =
					string(controlplanev1alpha4.NestedEtcdUnready)
				log.V(5).Info("The corresponding statefulset is not found, " +
					"will mark the NestedEtcd as unready")
				if err := r.Status().Update(ctx, &netcd); err != nil {
					log.Error(err, "fail to update the status of the NestedEtcd Object")
					return ctrl.Result{}, err
				}
			}
			// the statefulset is not found, create one
			if err := createNestedEtcdStatefulSet(ctx,
				r.Client, netcd, owner.Name, log); err != nil {
				log.Error(err, "fail to create NestedEtcd StatefulSet")
				return ctrl.Result{}, err
			}
			log.Info("successfully create the NestedEtcd StatefulSet")
			return ctrl.Result{}, nil
		}
		log.Error(err, "fail to get NestedEtcd StatefulSet")
		return ctrl.Result{}, err
	}

	if netcdSts.Status.ReadyReplicas == netcdSts.Status.Replicas {
		log.Info("The NestedEtcd StatefulSet is ready")
		if !IsNetcdReady(netcd.Status) {
			// As the NestedEtcd StatefulSet is ready, update NestedEtcd status
			ip, err := getNestedEtcdSvcClusterIP(ctx, r.Client, netcd)
			if err != nil {
				log.Error(err, "fail to get NestedEtcd Service ClusterIP")
				return ctrl.Result{}, err
			}
			netcd.Status.Phase = string(controlplanev1alpha4.NestedEtcdReady)
			netcd.Status.Addresses = []controlplanev1alpha4.NestedEtcdAddress{
				{
					IP:   ip,
					Port: 2379,
				},
			}
			log.V(5).Info("The corresponding statefulset is ready, " +
				"will mark the NestedEtcd as ready")
			if err := r.Status().Update(ctx, &netcd); err != nil {
				log.Error(err, "fail to update NestedEtcd Object")
				return ctrl.Result{}, err
			}
			log.Info("Successfully set the NestedEtcd object to ready",
				"address", netcd.Status.Addresses)
		}
		return ctrl.Result{}, nil
	}

	// As the NestedEtcd StatefulSet is unready, mark the NestedEtcd as unready
	// if its current status is ready
	if IsNetcdReady(netcd.Status) {
		netcd.Status.Phase = string(controlplanev1alpha4.NestedEtcdUnready)
		if err := r.Status().Update(ctx, &netcd); err != nil {
			log.Error(err, "fail to update NestedEtcd Object")
			return ctrl.Result{}, err
		}
		log.Info("Successfully set the NestedEtcd object to unready")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NestedEtcdReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(),
		&appsv1.StatefulSet{},
		statefulsetOwnerKey,
		func(rawObj ctrlcli.Object) []string {
			// grab the statefulset object, extract the owner
			sts := rawObj.(*appsv1.StatefulSet)
			owner := metav1.GetControllerOf(sts)
			if owner == nil {
				return nil
			}
			// make sure it's a NestedEtcd
			if owner.APIVersion != apiGVStr || owner.Kind != "NestedEtcd" {
				return nil
			}

			// and if so, return it
			return []string{owner.Name}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1alpha4.NestedEtcd{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

func genServiceObject(netcd controlplanev1alpha4.NestedEtcd,
	clusterName, templateURL string,
	log logr.Logger) (netcdSvc corev1.Service, retErr error) {
	defaultSvcTmpl, err := fetchTemplate(defaultServiceURL)
	if err != nil {
		retErr = fmt.Errorf("fail to fetch the default template "+
			"for the nestedetcd service: %v", err)
		return
	}
	svcStr, err := substituteTemplate(map[string]string{
		"nestedetcdName":        netcd.GetName(),
		"nestedetcdNamespace":   netcd.GetNamespace(),
		"nestedetcdStsReplicas": strconv.FormatInt(int64(netcd.Spec.Replicas), 10),
	}, defaultSvcTmpl)
	if err != nil {
		retErr = fmt.Errorf("fail to substitute the default template "+
			"for the nestedetcd Service: %v", err)
		return
	}
	rawSvcObj, err := yamlToObject([]byte(svcStr))
	if err != nil {
		retErr = fmt.Errorf("fail to convert yaml file to Serivce: %v", err)
		return
	}
	log.Info("deserialize yaml to runtime object(Service)")
	svcObj, ok := rawSvcObj.(*corev1.Service)
	if !ok {
		retErr = fmt.Errorf("fail to convert runtime object to Serivce")
		return
	}
	netcdSvc = *svcObj
	log.Info("convert runtime object to Service.")
	return
}

func genStatefulSetObject(netcd controlplanev1alpha4.NestedEtcd,
	clusterName, templateURL string,
	log logr.Logger) (netcdSts appsv1.StatefulSet, retErr error) {
	// 1 fetch the statefulset template
	defaultStsTmpl, err := fetchTemplate(defaultStatefulSetURL)
	if err != nil {
		retErr = fmt.Errorf("fail to fetch the default template "+
			"for the nestedetcd StatefulSet: %v", err)
		return
	}
	// 2 substitute the statefulset template
	stsStr, err := substituteTemplate(map[string]string{
		"nestedetcdName":         netcd.GetName(),
		"nestedetcdNamespace":    netcd.GetNamespace(),
		"nestedControlPlaneName": clusterName,
	}, defaultStsTmpl)
	if err != nil {
		retErr = fmt.Errorf("fail to substitute the default template "+
			"for the nestedetcd StatefulSet: %v", err)
		return
	}
	// 3 deserialize the yaml string to the StatefulSet object
	rawObj, err := yamlToObject([]byte(stsStr))
	if err != nil {
		retErr = fmt.Errorf("fail to convert yaml file to StatefulSet: %v", err)
		return
	}
	log.V(5).Info("deserialize yaml to runtime object(StatefulSet)")
	// 4 convert runtime Object to StatefulSet
	stsObj, ok := rawObj.(*appsv1.StatefulSet)
	if !ok {
		retErr = fmt.Errorf("fail to convert runtime object to StatefulSet")
		return
	}
	netcdSts = *stsObj
	log.V(5).Info("convert runtime object to StatefulSet.")
	// 5 apply NestedEtcd.Spec.Resources and NestedEtcd.Spec.Replicas
	// to the NestedEtcd StatefulSet
	for i := range netcdSts.Spec.Template.Spec.Containers {
		netcdSts.Spec.Template.Spec.Containers[i].Resources =
			netcd.Spec.Resources
	}
	if netcd.Spec.Replicas != 0 {
		*netcdSts.Spec.Replicas = netcd.Spec.Replicas
	}
	log.V(5).Info("The NestedEtcd StatefulSet's Resources and "+
		"Replicas fields are set",
		"StatefulSet", netcdSts.GetName())

	// 6 set the "--initial-cluster" command line flag for the Etcd container
	icaVal := genInitialClusterArgs(1, netcd.GetName(), netcd.GetName())
	stsArgs := append(netcdSts.Spec.Template.Spec.Containers[0].Args,
		"--initial-cluster", icaVal)
	netcdSts.Spec.Template.Spec.Containers[0].Args = stsArgs
	log.V(5).Info("The '--initial-cluster' command line option is set")

	// 7 TODO validate the patch and apply it to the template.
	return netcdSts, nil
}

func createNestedEtcdStatefulSet(ctx context.Context,
	cli ctrlcli.Client, netcd controlplanev1alpha4.NestedEtcd,
	clusterName string, log logr.Logger) error {
	var (
		netcdSts appsv1.StatefulSet
		netcdSvc corev1.Service
		err      error
	)
	// 1. Using the template defined by version/channel to create the
	// StatefulSet and the Service
	// TODO check the template version/channel, if not set, use the default.
	if netcd.Spec.Version == "" && netcd.Spec.Channel == "" {
		log.V(4).Info("The Version and Channel are not set, " +
			"will use the default template.")
		netcdSts, err = genStatefulSetObject(netcd, clusterName,
			defaultStatefulSetURL, log)
		if err != nil {
			return fmt.Errorf("fail to generate the "+
				"NestedEtcd Statefulset object: %v", err)
		}

		netcdSvc, err = genServiceObject(netcd, clusterName,
			defaultServiceURL, log)
		if err != nil {
			return fmt.Errorf("fail to generate the "+
				"NesteEtcd Service object: %v", err)
		}

	} else {
		panic("NOT IMPLEMENT YET")
	}
	// 2. set the NestedEtcd object as the owner of the StatefulSet
	or := metav1.NewControllerRef(&netcd,
		controlplanev1alpha4.GroupVersion.WithKind("NestedEtcd"))
	netcdSts.OwnerReferences = append(netcdSts.OwnerReferences, *or)

	if err := cli.Create(ctx, &netcdSvc); err != nil {
		return err
	}
	log.Info("successfully create the service for NestedEtcd StatefulSet")
	// 3. as we need to use the assigned ClusterIP to generate the TLS
	// certificate, we get the latest NestedEtcd Service object
	if err := cli.Get(ctx, types.NamespacedName{
		Name:      netcdSvc.GetName(),
		Namespace: netcdSvc.GetNamespace(),
	}, &netcdSvc); err != nil {
		return err
	}
	log.Info("get the latest Service")

	// 4. create the NestedEtcd StatefulSet
	return cli.Create(ctx, &netcdSts)
}

func IsNetcdReady(status controlplanev1alpha4.NestedEtcdStatus) bool {
	return status.Phase == string(controlplanev1alpha4.NestedEtcdReady)
}

func getNestedEtcdSvcClusterIP(ctx context.Context, cli ctrlcli.Client,
	netcd controlplanev1alpha4.NestedEtcd) (string, error) {
	var svc corev1.Service
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      netcd.GetName(),
	}, &svc); err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

func yamlToObject(yamlContent []byte) (runtime.Object, error) {
	decode := serializer.NewCodecFactory(scheme.Scheme).
		UniversalDeserializer().Decode
	obj, _, err := decode(yamlContent, nil, nil)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func substituteTemplate(context interface{}, tmpl string) (string, error) {
	t, tmplPrsErr := template.New("test").
		Option("missingkey=zero").Parse(tmpl)
	if tmplPrsErr != nil {
		return "", tmplPrsErr
	}
	writer := bytes.NewBuffer([]byte{})
	if err := t.Execute(writer, context); nil != err {
		return "", err
	}

	return writer.String(), nil
}

func fetchTemplate(templateURL string) (string, error) {
	rep, err := http.Get(templateURL)
	if err != nil {
		return "", err
	}

	defer rep.Body.Close()

	bodyBytes, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

func getOwner(netcd controlplanev1alpha4.NestedEtcd) metav1.OwnerReference {
	owners := netcd.GetOwnerReferences()
	if len(owners) == 0 {
		return metav1.OwnerReference{}
	}
	for _, owner := range owners {
		if owner.APIVersion == apiGVStr && owner.Kind == "NestedControlPlane" {
			return owner
		}
	}
	return metav1.OwnerReference{}
}

// genInitialClusterArgs generates the values for `--inital-cluster` option of
// etcd based on the number of replicas specified in etcd StatefulSet
func genInitialClusterArgs(replicas int32,
	stsName, svcName string) (argsVal string) {
	for i := int32(0); i < replicas; i++ {
		// use 2380 as the default port for etcd peer communication
		peerAddr := fmt.Sprintf("%s-%d=https://%s-%d.%s:%d",
			stsName, i, stsName, i, svcName, 2380)
		if i == replicas-1 {
			argsVal = argsVal + peerAddr
			break
		}
		argsVal = argsVal + peerAddr + ","
	}

	return argsVal
}
