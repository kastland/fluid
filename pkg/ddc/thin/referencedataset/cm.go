/*
  Copyright 2022 The Fluid Authors.

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

package referencedataset

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	datav1alpha1 "github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/common"
	"github.com/fluid-cloudnative/fluid/pkg/ddc/base"
	"github.com/fluid-cloudnative/fluid/pkg/utils"
	"github.com/fluid-cloudnative/fluid/pkg/utils/kubeclient"
)

func copyFuseDaemonSetForRefDataset(client client.Client, refDataset *datav1alpha1.Dataset, mountedRuntime base.RuntimeInfoInterface) error {
	var fuseName string
	switch mountedRuntime.GetRuntimeType() {
	case common.JindoRuntime:
		fuseName = mountedRuntime.GetName() + "-" + common.JindoChartName + "-fuse"
	default:
		fuseName = mountedRuntime.GetName() + "-fuse"
	}
	ds, err := kubeclient.GetDaemonset(client, fuseName, mountedRuntime.GetNamespace())
	if err != nil {
		return err
	}

	// copy fuse daemonset to refDataset's namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: refDataset.APIVersion,
		Kind:       refDataset.Kind,
		Name:       refDataset.Name,
		UID:        refDataset.UID,
	}

	dsToCreate := &appsv1.DaemonSet{}
	dsToCreate.Name = refDataset.Name + "-fuse"
	dsToCreate.Namespace = refDataset.Namespace
	dsToCreate.OwnerReferences = append(dsToCreate.OwnerReferences, ownerReference)
	dsToCreate.Spec = *ds.Spec.DeepCopy()
	if len(dsToCreate.Spec.Template.Spec.NodeSelector) == 0 {
		dsToCreate.Spec.Template.Spec.NodeSelector = map[string]string{}
	}
	dsToCreate.Spec.Template.Spec.NodeSelector["fluid.io/fuse-balloon"] = "true"

	err = client.Create(context.TODO(), dsToCreate)
	if utils.IgnoreAlreadyExists(err) != nil {
		return err
	}

	return nil
}

func createConfigMapForRefDataset(client client.Client, refDataset *datav1alpha1.Dataset, mountedRuntime base.RuntimeInfoInterface) error {
	mountedRuntimeType := mountedRuntime.GetRuntimeType()
	mountedRuntimeName := mountedRuntime.GetName()
	mountedRuntimeNamespace := mountedRuntime.GetNamespace()

	refNameSpace := refDataset.GetNamespace()

	// add owner reference to ensure config map deleted when delete the dataset
	ownerReference := metav1.OwnerReference{
		APIVersion: refDataset.APIVersion,
		Kind:       refDataset.Kind,
		Name:       refDataset.Name,
		UID:        refDataset.UID,
	}

	// copy the configmap to ref namespace.
	// TODO: any other config resource like secret need copied ?

	// Note: values configmap is not needed for fuse sidecar container.

	// TODO: decoupling the switch-case, too fragile
	switch mountedRuntimeType {
	// TODO:  currently the dst configmap name is the same as src configmap name to avoid modify the fuse init container filed,
	//       but duplicated name error can occurs if the dst namespace has same named runtime.
	case common.AlluxioRuntime:
		configMapName := mountedRuntimeName + "-config"
		err := kubeclient.CopyConfigMap(client, types.NamespacedName{Name: configMapName, Namespace: mountedRuntimeNamespace},
			types.NamespacedName{Name: configMapName, Namespace: refNameSpace}, ownerReference)
		if err != nil {
			return err
		}
	case common.JuiceFSRuntime:
		fuseScriptConfigMapName := mountedRuntimeName + "-fuse-script"
		err := kubeclient.CopyConfigMap(client, types.NamespacedName{Name: fuseScriptConfigMapName, Namespace: mountedRuntimeNamespace},
			types.NamespacedName{Name: fuseScriptConfigMapName, Namespace: refNameSpace}, ownerReference)
		if err != nil {
			return err
		}
	case common.GooseFSRuntime:
		configMapName := mountedRuntimeName + "-config"
		err := kubeclient.CopyConfigMap(client, types.NamespacedName{Name: configMapName, Namespace: mountedRuntimeNamespace},
			types.NamespacedName{Name: configMapName, Namespace: refNameSpace}, ownerReference)
		if err != nil {
			return err
		}
	case common.JindoRuntime:
		clientConfigMapName := mountedRuntimeName + "-jindofs-client-config"
		err := kubeclient.CopyConfigMap(client, types.NamespacedName{Name: clientConfigMapName, Namespace: mountedRuntimeNamespace},
			types.NamespacedName{Name: clientConfigMapName, Namespace: refNameSpace}, ownerReference)
		if err != nil {
			return err
		}
		configMapName := mountedRuntimeName + "-jindofs-config"
		err = kubeclient.CopyConfigMap(client, types.NamespacedName{Name: configMapName, Namespace: mountedRuntimeNamespace},
			types.NamespacedName{Name: configMapName, Namespace: refNameSpace}, ownerReference)
		if err != nil {
			return err
		}
	case common.ThinRuntime:
		runtimesetConfigMapName := mountedRuntimeName + "-runtimeset"
		err := kubeclient.CopyConfigMap(client, types.NamespacedName{Name: runtimesetConfigMapName, Namespace: mountedRuntimeNamespace},
			types.NamespacedName{Name: runtimesetConfigMapName, Namespace: refNameSpace}, ownerReference)
		if err != nil {
			return err
		}
	default:
		err := fmt.Errorf("fail to get configmap for runtime type: %s", mountedRuntimeType)
		return err
	}

	return nil
}
