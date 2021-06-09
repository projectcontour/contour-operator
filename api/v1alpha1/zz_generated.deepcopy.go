// +build !ignore_autogenerated

// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSLoadBalancerParameters) DeepCopyInto(out *AWSLoadBalancerParameters) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSLoadBalancerParameters.
func (in *AWSLoadBalancerParameters) DeepCopy() *AWSLoadBalancerParameters {
	if in == nil {
		return nil
	}
	out := new(AWSLoadBalancerParameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerPort) DeepCopyInto(out *ContainerPort) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerPort.
func (in *ContainerPort) DeepCopy() *ContainerPort {
	if in == nil {
		return nil
	}
	out := new(ContainerPort)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Contour) DeepCopyInto(out *Contour) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Contour.
func (in *Contour) DeepCopy() *Contour {
	if in == nil {
		return nil
	}
	out := new(Contour)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Contour) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContourList) DeepCopyInto(out *ContourList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Contour, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContourList.
func (in *ContourList) DeepCopy() *ContourList {
	if in == nil {
		return nil
	}
	out := new(ContourList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ContourList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContourNodePlacement) DeepCopyInto(out *ContourNodePlacement) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContourNodePlacement.
func (in *ContourNodePlacement) DeepCopy() *ContourNodePlacement {
	if in == nil {
		return nil
	}
	out := new(ContourNodePlacement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContourSpec) DeepCopyInto(out *ContourSpec) {
	*out = *in
	out.Namespace = in.Namespace
	in.NetworkPublishing.DeepCopyInto(&out.NetworkPublishing)
	if in.GatewayClassRef != nil {
		in, out := &in.GatewayClassRef, &out.GatewayClassRef
		*out = new(string)
		**out = **in
	}
	if in.IngressClassName != nil {
		in, out := &in.IngressClassName, &out.IngressClassName
		*out = new(string)
		**out = **in
	}
	if in.NodePlacement != nil {
		in, out := &in.NodePlacement, &out.NodePlacement
		*out = new(NodePlacement)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContourSpec.
func (in *ContourSpec) DeepCopy() *ContourSpec {
	if in == nil {
		return nil
	}
	out := new(ContourSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContourStatus) DeepCopyInto(out *ContourStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContourStatus.
func (in *ContourStatus) DeepCopy() *ContourStatus {
	if in == nil {
		return nil
	}
	out := new(ContourStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvoyNetworkPublishing) DeepCopyInto(out *EnvoyNetworkPublishing) {
	*out = *in
	in.LoadBalancer.DeepCopyInto(&out.LoadBalancer)
	if in.NodePorts != nil {
		in, out := &in.NodePorts, &out.NodePorts
		*out = make([]NodePort, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ContainerPorts != nil {
		in, out := &in.ContainerPorts, &out.ContainerPorts
		*out = make([]ContainerPort, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvoyNetworkPublishing.
func (in *EnvoyNetworkPublishing) DeepCopy() *EnvoyNetworkPublishing {
	if in == nil {
		return nil
	}
	out := new(EnvoyNetworkPublishing)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvoyNodePlacement) DeepCopyInto(out *EnvoyNodePlacement) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvoyNodePlacement.
func (in *EnvoyNodePlacement) DeepCopy() *EnvoyNodePlacement {
	if in == nil {
		return nil
	}
	out := new(EnvoyNodePlacement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoadBalancerStrategy) DeepCopyInto(out *LoadBalancerStrategy) {
	*out = *in
	in.ProviderParameters.DeepCopyInto(&out.ProviderParameters)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoadBalancerStrategy.
func (in *LoadBalancerStrategy) DeepCopy() *LoadBalancerStrategy {
	if in == nil {
		return nil
	}
	out := new(LoadBalancerStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceSpec) DeepCopyInto(out *NamespaceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceSpec.
func (in *NamespaceSpec) DeepCopy() *NamespaceSpec {
	if in == nil {
		return nil
	}
	out := new(NamespaceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkPublishing) DeepCopyInto(out *NetworkPublishing) {
	*out = *in
	in.Envoy.DeepCopyInto(&out.Envoy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkPublishing.
func (in *NetworkPublishing) DeepCopy() *NetworkPublishing {
	if in == nil {
		return nil
	}
	out := new(NetworkPublishing)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePlacement) DeepCopyInto(out *NodePlacement) {
	*out = *in
	if in.Contour != nil {
		in, out := &in.Contour, &out.Contour
		*out = new(ContourNodePlacement)
		(*in).DeepCopyInto(*out)
	}
	if in.Envoy != nil {
		in, out := &in.Envoy, &out.Envoy
		*out = new(EnvoyNodePlacement)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodePlacement.
func (in *NodePlacement) DeepCopy() *NodePlacement {
	if in == nil {
		return nil
	}
	out := new(NodePlacement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePort) DeepCopyInto(out *NodePort) {
	*out = *in
	if in.PortNumber != nil {
		in, out := &in.PortNumber, &out.PortNumber
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodePort.
func (in *NodePort) DeepCopy() *NodePort {
	if in == nil {
		return nil
	}
	out := new(NodePort)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProviderLoadBalancerParameters) DeepCopyInto(out *ProviderLoadBalancerParameters) {
	*out = *in
	if in.AWS != nil {
		in, out := &in.AWS, &out.AWS
		*out = new(AWSLoadBalancerParameters)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProviderLoadBalancerParameters.
func (in *ProviderLoadBalancerParameters) DeepCopy() *ProviderLoadBalancerParameters {
	if in == nil {
		return nil
	}
	out := new(ProviderLoadBalancerParameters)
	in.DeepCopyInto(out)
	return out
}
