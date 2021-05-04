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
	"text/template"

	openuri "github.com/utahta/go-openuri"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha4"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

// createNestedComponentSts will create the StatefulSet that runs the
// NestedComponent
func createNestedComponentSts(ctx context.Context,
	cli ctrlcli.Client, ncMeta metav1.ObjectMeta,
	ncSpec clusterv1.NestedComponentSpec,
	ncKind clusterv1.ComponentKind,
	controlPlaneName, clusterName string, log logr.Logger) error {
	var (
		ncSts appsv1.StatefulSet
		ncSvc corev1.Service
		err   error
	)
	// Setup the ownerReferences for all objects
	or := metav1.NewControllerRef(&ncMeta,
		clusterv1.GroupVersion.WithKind(string(ncKind)))

	// 1. Using the template defined by version/channel to create the
	// StatefulSet and the Service
	// TODO check the template version/channel, if not set, use the default.
	if ncSpec.Version == "" && ncSpec.Channel == "" {
		log.V(4).Info("The Version and Channel are not set, " +
			"will use the default template.")
		ncSts, err = genStatefulSetObject(ncMeta, ncSpec, ncKind, controlPlaneName, clusterName, cli, log)
		if err != nil {
			return fmt.Errorf("fail to generate the Statefulset object: %v", err)
		}

		if ncKind != clusterv1.ControllerManager {
			// no need to create the service for the NestedControllerManager
			ncSvc, err = genServiceObject(ncMeta, ncSpec, ncKind, controlPlaneName, clusterName, log)
			ncSvc.SetOwnerReferences([]metav1.OwnerReference{*or})
			if err != nil {
				return fmt.Errorf("fail to generate the Service object: %v", err)
			}
			if err := cli.Create(ctx, &ncSvc); err != nil {
				return err
			}
			log.Info("successfully create the service for the StatefulSet",
				"component", ncKind)
		}

	} else {
		panic("NOT IMPLEMENT YET")
	}
	// 2. set the NestedComponent object as the owner of the StatefulSet
	ncSts.SetOwnerReferences([]metav1.OwnerReference{*or})

	// 4. create the NestedComponent StatefulSet
	return cli.Create(ctx, &ncSts)
}

// genServiceObject generates the Service object corresponding to the
// NestedComponent
func genServiceObject(ncMeta metav1.ObjectMeta,
	ncSpec clusterv1.NestedComponentSpec, ncKind clusterv1.ComponentKind,
	controlPlaneName, clusterName string, log logr.Logger) (ncSvc corev1.Service, retErr error) {
	var templateURL string
	if ncSpec.Version == "" && ncSpec.Channel == "" {
		switch ncKind {
		case clusterv1.APIServer:
			templateURL = defaultKASServiceURL
		case clusterv1.Etcd:
			templateURL = defaultEtcdServiceURL
		default:
			panic("Unreachable")
		}
	} else {
		panic("NOT IMPLEMENT YET")
	}
	svcTmpl, err := fetchTemplate(templateURL)
	if err != nil {
		retErr = fmt.Errorf("fail to fetch the default template "+
			"for the %s service: %v", ncKind, err)
		return
	}

	templateCtx := getTemplateArgs(ncMeta, controlPlaneName, clusterName)

	svcStr, err := substituteTemplate(templateCtx, svcTmpl)
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
	ncSvc = *svcObj
	log.Info("convert runtime object to Service.")
	return
}

// genStatefulSetObject generates the StatefulSet object corresponding to the
// NestedComponent
func genStatefulSetObject(
	ncMeta metav1.ObjectMeta,
	ncSpec clusterv1.NestedComponentSpec,
	ncKind clusterv1.ComponentKind, controlPlaneName, clusterName string,
	cli ctrlcli.Client, log logr.Logger) (ncSts appsv1.StatefulSet, retErr error) {
	var templateURL string
	if ncSpec.Version == "" && ncSpec.Channel == "" {
		log.V(4).Info("The Version and Channel are not set, " +
			"will use the default template.")
		switch ncKind {
		case clusterv1.APIServer:
			templateURL = defaultKASStatefulSetURL
		case clusterv1.Etcd:
			templateURL = defaultEtcdStatefulSetURL
		case clusterv1.ControllerManager:
			templateURL = defaultKCMStatefulSetURL
		default:
			panic("Unreachable")
		}
	} else {
		panic("NOT IMPLEMENT YET")
	}

	// 1 fetch the statefulset template
	stsTmpl, err := fetchTemplate(templateURL)
	if err != nil {
		retErr = fmt.Errorf("fail to fetch the default template "+
			"for the %s StatefulSet: %v", ncKind, err)
		return
	}
	// 2 substitute the statefulset template
	templateCtx := getTemplateArgs(ncMeta, controlPlaneName, clusterName)
	stsStr, err := substituteTemplate(templateCtx, stsTmpl)
	if err != nil {
		retErr = fmt.Errorf("fail to substitute the default template "+
			"for the %s StatefulSet: %v", ncKind, err)
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
	log.V(5).Info("convert runtime object to StatefulSet.")
	// 5 apply NestedComponent.Spec.Resources and NestedComponent.Spec.Replicas
	// to the NestedComponent StatefulSet
	for i := range stsObj.Spec.Template.Spec.Containers {
		stsObj.Spec.Template.Spec.Containers[i].Resources =
			ncSpec.Resources
	}
	if ncSpec.Replicas != 0 {
		stsObj.Spec.Replicas = &ncSpec.Replicas
	}
	log.V(5).Info("The NestedEtcd StatefulSet's Resources and "+
		"Replicas fields are set",
		"StatefulSet", stsObj.GetName())

	// 6 set the "--initial-cluster" command line flag for the Etcd container
	if ncKind == clusterv1.Etcd {
		icaVal := genInitialClusterArgs(1, clusterName, clusterName, ncMeta.GetNamespace())
		stsArgs := append(stsObj.Spec.Template.Spec.Containers[0].Args,
			"--initial-cluster", icaVal)
		stsObj.Spec.Template.Spec.Containers[0].Args = stsArgs
		log.V(5).Info("The '--initial-cluster' command line option is set")
	}

	// 7 TODO validate the patch and apply it to the template.
	ncSts = *stsObj
	return
}

func getTemplateArgs(ncMeta metav1.ObjectMeta, controlPlaneName, clusterName string) map[string]string {
	return map[string]string{
		"componentName":      ncMeta.GetName(),
		"componentNamespace": ncMeta.GetNamespace(),
		"clusterName":        clusterName,
		"controlPlaneName":   controlPlaneName,
	}
}

// yamlToObject deserialize the yaml to the runtime object
func yamlToObject(yamlContent []byte) (runtime.Object, error) {
	decode := serializer.NewCodecFactory(scheme.Scheme).
		UniversalDeserializer().Decode
	obj, _, err := decode(yamlContent, nil, nil)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// substituteTemplate substitutes the template contents with the context
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

// fetchTemplate fetches the component template through the tmplateURL
func fetchTemplate(templateURL string) (string, error) {
	rep, err := openuri.Open(templateURL)
	if err != nil {
		return "", err
	}
	defer rep.Close()

	bodyBytes, err := ioutil.ReadAll(rep)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// getOwner gets the ownerreference of the NestedComponent
func getOwner(ncMeta metav1.ObjectMeta) metav1.OwnerReference {
	owners := ncMeta.GetOwnerReferences()
	if len(owners) == 0 {
		return metav1.OwnerReference{}
	}
	for _, owner := range owners {
		if owner.APIVersion == clusterv1.GroupVersion.String() &&
			owner.Kind == "NestedControlPlane" {
			return owner
		}
	}
	return metav1.OwnerReference{}
}

// genAPIServerSvcRef generates the ObjectReference that points to the
// APISrver service
func genAPIServerSvcRef(cli ctrlcli.Client,
	nkas clusterv1.NestedAPIServer, clusterName string) (corev1.ObjectReference, error) {
	var (
		svc    corev1.Service
		objRef corev1.ObjectReference
	)
	if err := cli.Get(context.TODO(), types.NamespacedName{
		Namespace: nkas.GetNamespace(),
		Name:      fmt.Sprintf("%s-apiserver", clusterName),
	}, &svc); err != nil {
		return objRef, err
	}
	objRef = genObjRefFromObj(&svc)
	return objRef, nil
}

// genObjRefFromObj generates the ObjectReference of the given object
func genObjRefFromObj(obj ctrlcli.Object) corev1.ObjectReference {
	return corev1.ObjectReference{
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().Version,
	}
}

func IsComponentReady(status addonv1alpha1.CommonStatus) bool {
	return status.Phase == string(clusterv1.Ready)
}
