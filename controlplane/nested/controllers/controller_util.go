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

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/kubeadm"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

// +kubebuilder:rbac:groups="";apps,resources=services;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="";apps,resources=services/status;statefulsets/status,verbs=get;update;patch

// createNestedComponentSts will create the StatefulSet that runs the
// NestedComponent.
func createNestedComponentSts(ctx context.Context,
	cli ctrlcli.Client, ncMeta metav1.ObjectMeta,
	ncSpec controlplanev1.NestedComponentSpec,
	ncKind, clusterName string, log logr.Logger) error {
	// setup the ownerReferences for all objects
	or := metav1.NewControllerRef(&ncMeta,
		controlplanev1.GroupVersion.WithKind(ncKind))

	ncSts, err := genStatefulSetObject(cli, ncMeta, ncSpec, ncKind, clusterName, log)
	if err != nil {
		return errors.Errorf("fail to generate the Statefulset object: %v", err)
	}

	if ncKind != kubeadm.ControllerManager {
		// no need to create the service for the NestedControllerManager
		ncSvc, err := genServiceObject(ncKind, clusterName, ncMeta.GetName(), ncMeta.GetNamespace())
		if err != nil {
			return errors.Errorf("fail to generate the Service object: %v", err)
		}

		ncSvc.SetOwnerReferences([]metav1.OwnerReference{*or})
		if err := cli.Create(ctx, ncSvc); err != nil {
			return err
		}
		log.Info("successfully create the service for the StatefulSet",
			"component", ncKind)
	}

	// set the NestedComponent object as the owner of the StatefulSet
	ncSts.SetOwnerReferences([]metav1.OwnerReference{*or})

	// create the NestedComponent StatefulSet
	return cli.Create(ctx, ncSts)
}

// genServiceObject generates the Service object corresponding to the NestedComponent.
func genServiceObject(ncKind, clusterName, componentName, componentNamespace string) (*corev1.Service, error) {
	switch ncKind {
	case kubeadm.APIServer:
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName + "-apiserver",
				Namespace: componentNamespace,
				Labels: map[string]string{
					"component-name": componentName,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"component-name": componentName,
				},
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "api",
						Port:       6443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromString("api"),
					},
				},
			},
		}, nil
	case kubeadm.Etcd:
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName + "-etcd",
				Namespace: componentNamespace,
				Labels: map[string]string{
					"component-name": componentName,
				},
			},
			Spec: corev1.ServiceSpec{
				PublishNotReadyAddresses: true,
				ClusterIP:                "None",
				Selector: map[string]string{
					"component-name": componentName,
				},
			},
		}, nil
	default:
		return nil, errors.Errorf("unknown component type: %s", ncKind)
	}
}

// objectToYaml serialize the runtime object to the yaml.
func objectToYaml(obj runtime.Object) ([]byte, error) {
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	buf := bytes.NewBuffer([]byte{})
	if err := serializer.Encode(obj, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// genStatefulSetManifest complete the pod spec and use it to generate the
// statefulset manifest.
func genStatefulSetManifest(podManifest, ncKind, clusterName, componentName, componentNamespace string) (*appsv1.StatefulSet, error) {
	pod := corev1.Pod{}
	if err := yamlToObject([]byte(podManifest), &pod); err != nil {
		return nil, errors.Errorf("fail to convert yaml file to pod: %v", err)
	}
	switch ncKind {
	case kubeadm.APIServer:
	case kubeadm.Etcd:
	case kubeadm.ControllerManager:
	default:
		return nil, errors.Errorf("invalid component type: %s", ncKind)
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-" + ncKind,
			Namespace: componentNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: clusterName + "-" + ncKind,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component-name": componentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"component-name": componentName,
					},
				},
				Spec: pod.Spec,
			},
		},
	}, nil
}

// genStatefulSetObject generates the StatefulSet object corresponding to the NestedComponent.
func genStatefulSetObject(
	cli ctrlcli.Client,
	ncMeta metav1.ObjectMeta,
	ncSpec controlplanev1.NestedComponentSpec,
	ncKind, clusterName string,
	log logr.Logger) (*appsv1.StatefulSet, error) {
	cm := corev1.ConfigMap{}
	if err := cli.Get(context.TODO(), types.NamespacedName{
		Namespace: ncMeta.Namespace,
		Name:      clusterName + "-" + kubeadm.ManifestsConfigmapSuffix,
	}, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "manifests configmap not found")
		}
		return nil, err
	}

	// 1. get the pod spec
	podManifest := cm.Data[ncKind]
	if podManifest == "" {
		return nil, errors.Errorf("data %s is not found", ncKind)
	}
	// 2. generate the statefulset manifest
	ncSts, err := genStatefulSetManifest(podManifest, ncKind, clusterName, ncMeta.GetName(), ncMeta.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate the statefulset manifest")
	}

	// 3. apply NestedComponent.Spec.Resources and NestedComponent.Spec.Replicas
	// to the NestedComponent StatefulSet
	for i := range ncSts.Spec.Template.Spec.Containers {
		ncSts.Spec.Template.Spec.Containers[i].Resources =
			ncSpec.Resources
	}
	if ncSpec.Replicas != 0 {
		ncSts.Spec.Replicas = &ncSpec.Replicas
	}
	log.V(5).Info("The NestedEtcd StatefulSet's Resources and "+
		"Replicas fields are set",
		"StatefulSet", ncSts.GetName())

	// 4. set the "--initial-cluster" command line flag for the Etcd container
	if ncKind == kubeadm.Etcd {
		icaVal := genInitialClusterArgs(1, clusterName, clusterName, ncMeta.GetNamespace())
		ncSts.Spec.Template.Spec.Containers[0].Command = append(
			ncSts.Spec.Template.Spec.Containers[0].Command,
			fmt.Sprintf("--initial-cluster=%s", icaVal))
		log.V(5).Info("The '--initial-cluster' command line option is set")
	}
	return ncSts, nil
}

// yamlToObject deserialize the yaml to the runtime object.
func yamlToObject(yamlContent []byte, obj runtime.Object) error {
	decode := serializer.NewCodecFactory(scheme.Scheme).
		UniversalDeserializer().Decode
	_, _, err := decode(yamlContent, nil, obj)
	if err != nil {
		return err
	}
	return nil
}

// substituteTemplate substitutes the template contents with the context.
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

// getOwner gets the ownerreference of the NestedComponent.
func getOwner(ncMeta metav1.ObjectMeta) metav1.OwnerReference {
	owners := ncMeta.GetOwnerReferences()
	if len(owners) == 0 {
		return metav1.OwnerReference{}
	}
	for _, owner := range owners {
		if owner.APIVersion == controlplanev1.GroupVersion.String() &&
			owner.Kind == "NestedControlPlane" {
			return owner
		}
	}
	return metav1.OwnerReference{}
}

// genAPIServerSvcRef generates the ObjectReference that points to the
// APISrver service.
func genAPIServerSvcRef(cli ctrlcli.Client,
	nkas controlplanev1.NestedAPIServer, clusterName string) (corev1.ObjectReference, error) {
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

// genObjRefFromObj generates the ObjectReference of the given object.
func genObjRefFromObj(obj ctrlcli.Object) corev1.ObjectReference {
	return corev1.ObjectReference{
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().Version,
	}
}

// IsComponentReady will return bool if status Ready.
func IsComponentReady(status addonv1alpha1.CommonStatus) bool {
	return status.Phase == string(controlplanev1.Ready)
}

// createManifestsConfigMap create the configmap that holds the manifests of
// the NestedComponent. NOTE this function will be deprecated once the
// nestedmachine_controller is implemented.
func createManifestsConfigMap(cli ctrlcli.Client, manifests map[string]corev1.Pod, clusterName, namespace string) error {
	data := map[string]string{}
	for name, pod := range manifests {
		tmpPod := pod
		podYaml, err := objectToYaml(&tmpPod)
		if err != nil {
			return err
		}
		data[name] = string(podYaml)
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName + "-" + kubeadm.ManifestsConfigmapSuffix,
		},
		Data: data,
	}
	return cli.Create(context.TODO(), &cm)
}

// completeTemplates completes the pod templates of nested control plane
// components.
func completeTemplates(templates map[string]string, clusterName string) (map[string]corev1.Pod, error) {
	var ret map[string]corev1.Pod = make(map[string]corev1.Pod)
	for name, podTemplate := range templates {
		pod := corev1.Pod{}
		if err := yamlToObject([]byte(podTemplate), &pod); err != nil {
			return nil, err
		}
		switch name {
		case kubeadm.APIServer:
			ret[kubeadm.APIServer] = completeKASPodSpec(pod, clusterName)
		case kubeadm.ControllerManager:
			ret[kubeadm.ControllerManager] = completeKCMPodSpec(pod, clusterName)
		case kubeadm.Etcd:
			ret[kubeadm.Etcd] = completeEtcdPodSpec(pod, clusterName)
		default:
			return nil, errors.New("unknown component: " + name)
		}
	}
	return ret, nil
}

// completeKASPodSpec sets volumes, envs and other fields for the kube-apiserver pod spec.
func completeKASPodSpec(pod corev1.Pod, clusterName string) corev1.Pod {
	ps := pod.Spec
	pod.Spec.DNSConfig = &corev1.PodDNSConfig{
		Searches: []string{"cluster.local"},
	}
	ps.Hostname = kubeadm.APIServer
	ps.Subdomain = clusterName + "-apiserver"
	ps.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
	ps.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/kubernetes/pki/etcd/ca",
			Name:      clusterName + "-etcd-ca",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/etcd",
			Name:      clusterName + "-etcd-client",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/apiserver/ca",
			Name:      clusterName + "-ca",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/apiserver",
			Name:      clusterName + "-apiserver-client",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/kubelet",
			Name:      clusterName + "-kubelet-client",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/service-account",
			Name:      clusterName + "-sa",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/proxy/ca",
			Name:      clusterName + "-proxy",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/proxy",
			Name:      clusterName + "-proxy-client",
			ReadOnly:  true,
		},
	}
	ps.Containers[0].Ports = append(ps.Containers[0].Ports, corev1.ContainerPort{
		Name:          "api",
		ContainerPort: 6443,
	})
	ps.Containers[0].LivenessProbe.Handler.HTTPGet.Host = loopbackAddress
	ps.Containers[0].ReadinessProbe.Handler.HTTPGet.Host = loopbackAddress
	ps.Containers[0].StartupProbe.Handler.HTTPGet.Host = loopbackAddress

	// disable the hostnetwork
	ps.HostNetwork = false
	var volSrtMode int32 = 420
	ps.Volumes = []corev1.Volume{
		{
			Name: clusterName + "-apiserver-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-apiserver-client",
				},
			},
		},
		{
			Name: clusterName + "-etcd-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-etcd",
				},
			},
		},
		{
			Name: clusterName + "-etcd-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-etcd-client",
				},
			},
		},
		{
			Name: clusterName + "-kubelet-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-kubelet-client",
				},
			},
		},
		{
			Name: clusterName + "-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-ca",
				},
			},
		},
		{
			Name: clusterName + "-sa",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-sa",
				},
			},
		},
		{
			Name: clusterName + "-proxy",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-proxy",
				},
			},
		},
		{
			Name: clusterName + "-proxy-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-proxy-client",
				},
			},
		},
	}
	pod.Spec = ps
	return pod
}

// completeKCMPodSpec sets volumes and other fields for the kube-controller-manager pod spec.
func completeKCMPodSpec(pod corev1.Pod, clusterName string) corev1.Pod {
	ps := pod.Spec
	ps.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/kubernetes/pki/root/ca",
			Name:      clusterName + "-ca",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/root",
			Name:      clusterName + "-apiserver-client",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/service-account",
			Name:      clusterName + "-sa",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/proxy/ca",
			Name:      clusterName + "-proxy",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/kubeconfig",
			Name:      clusterName + "-kubeconfig",
			ReadOnly:  true,
		},
	}

	// disable the hostnetwork
	ps.HostNetwork = false

	var volSrtMode int32 = 420
	ps.Volumes = []corev1.Volume{
		{
			Name: clusterName + "-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-ca",
				},
			},
		},
		{
			Name: clusterName + "-apiserver-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-apiserver-client",
				},
			},
		},
		{
			Name: clusterName + "-sa",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-sa",
				},
			},
		},
		{
			Name: clusterName + "-proxy",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-proxy",
				},
			},
		},
		{
			Name: clusterName + "-kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-kubeconfig",
					Items: []corev1.KeyToPath{
						{
							Key:  "value",
							Path: "controller-manager-kubeconfig",
						},
					},
				},
			},
		},
	}
	pod.Spec = ps
	return pod
}

// completeEtcdPodSpec sets volumes, envs and other fields for the etcd pod spec.
func completeEtcdPodSpec(pod corev1.Pod, clusterName string) corev1.Pod {
	ps := pod.Spec
	ps.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "HOSTNAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
	ps.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/kubernetes/pki/ca",
			Name:      clusterName + "-etcd-ca",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/etcd",
			Name:      clusterName + "-etcd-client",
			ReadOnly:  true,
		},
		{
			MountPath: "/etc/kubernetes/pki/health",
			Name:      clusterName + "-etcd-health-client",
			ReadOnly:  true,
		},
	}
	// remove the --initial-cluster option
	for i, cmd := range ps.Containers[0].Command {
		if strings.Contains(cmd, "initial-cluster") {
			ps.Containers[0].Command = append(ps.Containers[0].Command[:i], ps.Containers[0].Command[i+1:]...)
		}
	}
	// disable the hostnetwork
	ps.HostNetwork = false

	var volSrtMode int32 = 420
	ps.Volumes = []corev1.Volume{
		{
			Name: clusterName + "-etcd-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-etcd",
				},
			},
		},
		{
			Name: clusterName + "-etcd-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-etcd-client",
				},
			},
		},
		{
			Name: clusterName + "-etcd-health-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &volSrtMode,
					SecretName:  clusterName + "-etcd-health-client",
				},
			},
		},
	}
	pod.Spec = ps
	return pod
}
