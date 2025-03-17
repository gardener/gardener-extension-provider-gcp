//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by deepcopy-gen. DO NOT EDIT.

package gcp

import (
	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackupBucketConfig) DeepCopyInto(out *BackupBucketConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Immutability != nil {
		in, out := &in.Immutability, &out.Immutability
		*out = new(ImmutableConfig)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackupBucketConfig.
func (in *BackupBucketConfig) DeepCopy() *BackupBucketConfig {
	if in == nil {
		return nil
	}
	out := new(BackupBucketConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BackupBucketConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudControllerManagerConfig) DeepCopyInto(out *CloudControllerManagerConfig) {
	*out = *in
	if in.FeatureGates != nil {
		in, out := &in.FeatureGates, &out.FeatureGates
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudControllerManagerConfig.
func (in *CloudControllerManagerConfig) DeepCopy() *CloudControllerManagerConfig {
	if in == nil {
		return nil
	}
	out := new(CloudControllerManagerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudNAT) DeepCopyInto(out *CloudNAT) {
	*out = *in
	if in.EndpointIndependentMapping != nil {
		in, out := &in.EndpointIndependentMapping, &out.EndpointIndependentMapping
		*out = new(EndpointIndependentMapping)
		**out = **in
	}
	if in.MinPortsPerVM != nil {
		in, out := &in.MinPortsPerVM, &out.MinPortsPerVM
		*out = new(int32)
		**out = **in
	}
	if in.MaxPortsPerVM != nil {
		in, out := &in.MaxPortsPerVM, &out.MaxPortsPerVM
		*out = new(int32)
		**out = **in
	}
	if in.NatIPNames != nil {
		in, out := &in.NatIPNames, &out.NatIPNames
		*out = make([]NatIPName, len(*in))
		copy(*out, *in)
	}
	if in.IcmpIdleTimeoutSec != nil {
		in, out := &in.IcmpIdleTimeoutSec, &out.IcmpIdleTimeoutSec
		*out = new(int32)
		**out = **in
	}
	if in.TcpEstablishedIdleTimeoutSec != nil {
		in, out := &in.TcpEstablishedIdleTimeoutSec, &out.TcpEstablishedIdleTimeoutSec
		*out = new(int32)
		**out = **in
	}
	if in.TcpTimeWaitTimeoutSec != nil {
		in, out := &in.TcpTimeWaitTimeoutSec, &out.TcpTimeWaitTimeoutSec
		*out = new(int32)
		**out = **in
	}
	if in.TcpTransitoryIdleTimeoutSec != nil {
		in, out := &in.TcpTransitoryIdleTimeoutSec, &out.TcpTransitoryIdleTimeoutSec
		*out = new(int32)
		**out = **in
	}
	if in.UdpIdleTimeoutSec != nil {
		in, out := &in.UdpIdleTimeoutSec, &out.UdpIdleTimeoutSec
		*out = new(int32)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudNAT.
func (in *CloudNAT) DeepCopy() *CloudNAT {
	if in == nil {
		return nil
	}
	out := new(CloudNAT)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudProfileConfig) DeepCopyInto(out *CloudProfileConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.MachineImages != nil {
		in, out := &in.MachineImages, &out.MachineImages
		*out = make([]MachineImages, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudProfileConfig.
func (in *CloudProfileConfig) DeepCopy() *CloudProfileConfig {
	if in == nil {
		return nil
	}
	out := new(CloudProfileConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CloudProfileConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudRouter) DeepCopyInto(out *CloudRouter) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudRouter.
func (in *CloudRouter) DeepCopy() *CloudRouter {
	if in == nil {
		return nil
	}
	out := new(CloudRouter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneConfig) DeepCopyInto(out *ControlPlaneConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.CloudControllerManager != nil {
		in, out := &in.CloudControllerManager, &out.CloudControllerManager
		*out = new(CloudControllerManagerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = new(Storage)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneConfig.
func (in *ControlPlaneConfig) DeepCopy() *ControlPlaneConfig {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ControlPlaneConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataVolume) DeepCopyInto(out *DataVolume) {
	*out = *in
	if in.SourceImage != nil {
		in, out := &in.SourceImage, &out.SourceImage
		*out = new(string)
		**out = **in
	}
	if in.ProvisionedIops != nil {
		in, out := &in.ProvisionedIops, &out.ProvisionedIops
		*out = new(int64)
		**out = **in
	}
	if in.ProvisionedThroughput != nil {
		in, out := &in.ProvisionedThroughput, &out.ProvisionedThroughput
		*out = new(int64)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataVolume.
func (in *DataVolume) DeepCopy() *DataVolume {
	if in == nil {
		return nil
	}
	out := new(DataVolume)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiskEncryption) DeepCopyInto(out *DiskEncryption) {
	*out = *in
	if in.KmsKeyName != nil {
		in, out := &in.KmsKeyName, &out.KmsKeyName
		*out = new(string)
		**out = **in
	}
	if in.KmsKeyServiceAccount != nil {
		in, out := &in.KmsKeyServiceAccount, &out.KmsKeyServiceAccount
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiskEncryption.
func (in *DiskEncryption) DeepCopy() *DiskEncryption {
	if in == nil {
		return nil
	}
	out := new(DiskEncryption)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointIndependentMapping) DeepCopyInto(out *EndpointIndependentMapping) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointIndependentMapping.
func (in *EndpointIndependentMapping) DeepCopy() *EndpointIndependentMapping {
	if in == nil {
		return nil
	}
	out := new(EndpointIndependentMapping)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FlowLogs) DeepCopyInto(out *FlowLogs) {
	*out = *in
	if in.AggregationInterval != nil {
		in, out := &in.AggregationInterval, &out.AggregationInterval
		*out = new(string)
		**out = **in
	}
	if in.FlowSampling != nil {
		in, out := &in.FlowSampling, &out.FlowSampling
		*out = new(float64)
		**out = **in
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FlowLogs.
func (in *FlowLogs) DeepCopy() *FlowLogs {
	if in == nil {
		return nil
	}
	out := new(FlowLogs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GPU) DeepCopyInto(out *GPU) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GPU.
func (in *GPU) DeepCopy() *GPU {
	if in == nil {
		return nil
	}
	out := new(GPU)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImmutableConfig) DeepCopyInto(out *ImmutableConfig) {
	*out = *in
	out.RetentionPeriod = in.RetentionPeriod
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImmutableConfig.
func (in *ImmutableConfig) DeepCopy() *ImmutableConfig {
	if in == nil {
		return nil
	}
	out := new(ImmutableConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfrastructureConfig) DeepCopyInto(out *InfrastructureConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Networks.DeepCopyInto(&out.Networks)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfrastructureConfig.
func (in *InfrastructureConfig) DeepCopy() *InfrastructureConfig {
	if in == nil {
		return nil
	}
	out := new(InfrastructureConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfrastructureConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfrastructureState) DeepCopyInto(out *InfrastructureState) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Data != nil {
		in, out := &in.Data, &out.Data
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Routes != nil {
		in, out := &in.Routes, &out.Routes
		*out = make([]Route, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfrastructureState.
func (in *InfrastructureState) DeepCopy() *InfrastructureState {
	if in == nil {
		return nil
	}
	out := new(InfrastructureState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfrastructureState) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfrastructureStatus) DeepCopyInto(out *InfrastructureStatus) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Networks.DeepCopyInto(&out.Networks)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfrastructureStatus.
func (in *InfrastructureStatus) DeepCopy() *InfrastructureStatus {
	if in == nil {
		return nil
	}
	out := new(InfrastructureStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfrastructureStatus) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineImage) DeepCopyInto(out *MachineImage) {
	*out = *in
	if in.Architecture != nil {
		in, out := &in.Architecture, &out.Architecture
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineImage.
func (in *MachineImage) DeepCopy() *MachineImage {
	if in == nil {
		return nil
	}
	out := new(MachineImage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineImageVersion) DeepCopyInto(out *MachineImageVersion) {
	*out = *in
	if in.Architecture != nil {
		in, out := &in.Architecture, &out.Architecture
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineImageVersion.
func (in *MachineImageVersion) DeepCopy() *MachineImageVersion {
	if in == nil {
		return nil
	}
	out := new(MachineImageVersion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineImages) DeepCopyInto(out *MachineImages) {
	*out = *in
	if in.Versions != nil {
		in, out := &in.Versions, &out.Versions
		*out = make([]MachineImageVersion, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineImages.
func (in *MachineImages) DeepCopy() *MachineImages {
	if in == nil {
		return nil
	}
	out := new(MachineImages)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NatIP) DeepCopyInto(out *NatIP) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NatIP.
func (in *NatIP) DeepCopy() *NatIP {
	if in == nil {
		return nil
	}
	out := new(NatIP)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NatIPName) DeepCopyInto(out *NatIPName) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NatIPName.
func (in *NatIPName) DeepCopy() *NatIPName {
	if in == nil {
		return nil
	}
	out := new(NatIPName)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkConfig) DeepCopyInto(out *NetworkConfig) {
	*out = *in
	if in.VPC != nil {
		in, out := &in.VPC, &out.VPC
		*out = new(VPC)
		(*in).DeepCopyInto(*out)
	}
	if in.CloudNAT != nil {
		in, out := &in.CloudNAT, &out.CloudNAT
		*out = new(CloudNAT)
		(*in).DeepCopyInto(*out)
	}
	if in.Internal != nil {
		in, out := &in.Internal, &out.Internal
		*out = new(string)
		**out = **in
	}
	if in.FlowLogs != nil {
		in, out := &in.FlowLogs, &out.FlowLogs
		*out = new(FlowLogs)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkConfig.
func (in *NetworkConfig) DeepCopy() *NetworkConfig {
	if in == nil {
		return nil
	}
	out := new(NetworkConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkStatus) DeepCopyInto(out *NetworkStatus) {
	*out = *in
	in.VPC.DeepCopyInto(&out.VPC)
	if in.Subnets != nil {
		in, out := &in.Subnets, &out.Subnets
		*out = make([]Subnet, len(*in))
		copy(*out, *in)
	}
	if in.NatIPs != nil {
		in, out := &in.NatIPs, &out.NatIPs
		*out = make([]NatIP, len(*in))
		copy(*out, *in)
	}
	if in.IPFamilies != nil {
		in, out := &in.IPFamilies, &out.IPFamilies
		*out = make([]v1beta1.IPFamily, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkStatus.
func (in *NetworkStatus) DeepCopy() *NetworkStatus {
	if in == nil {
		return nil
	}
	out := new(NetworkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Route) DeepCopyInto(out *Route) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Route.
func (in *Route) DeepCopy() *Route {
	if in == nil {
		return nil
	}
	out := new(Route)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceAccount) DeepCopyInto(out *ServiceAccount) {
	*out = *in
	if in.Scopes != nil {
		in, out := &in.Scopes, &out.Scopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceAccount.
func (in *ServiceAccount) DeepCopy() *ServiceAccount {
	if in == nil {
		return nil
	}
	out := new(ServiceAccount)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
	if in.ManagedDefaultStorageClass != nil {
		in, out := &in.ManagedDefaultStorageClass, &out.ManagedDefaultStorageClass
		*out = new(bool)
		**out = **in
	}
	if in.ManagedDefaultVolumeSnapshotClass != nil {
		in, out := &in.ManagedDefaultVolumeSnapshotClass, &out.ManagedDefaultVolumeSnapshotClass
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Subnet) DeepCopyInto(out *Subnet) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Subnet.
func (in *Subnet) DeepCopy() *Subnet {
	if in == nil {
		return nil
	}
	out := new(Subnet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VPC) DeepCopyInto(out *VPC) {
	*out = *in
	if in.CloudRouter != nil {
		in, out := &in.CloudRouter, &out.CloudRouter
		*out = new(CloudRouter)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VPC.
func (in *VPC) DeepCopy() *VPC {
	if in == nil {
		return nil
	}
	out := new(VPC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	if in.LocalSSDInterface != nil {
		in, out := &in.LocalSSDInterface, &out.LocalSSDInterface
		*out = new(string)
		**out = **in
	}
	if in.Encryption != nil {
		in, out := &in.Encryption, &out.Encryption
		*out = new(DiskEncryption)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Volume.
func (in *Volume) DeepCopy() *Volume {
	if in == nil {
		return nil
	}
	out := new(Volume)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerConfig) DeepCopyInto(out *WorkerConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.GPU != nil {
		in, out := &in.GPU, &out.GPU
		*out = new(GPU)
		**out = **in
	}
	if in.Volume != nil {
		in, out := &in.Volume, &out.Volume
		*out = new(Volume)
		(*in).DeepCopyInto(*out)
	}
	if in.DataVolumes != nil {
		in, out := &in.DataVolumes, &out.DataVolumes
		*out = make([]DataVolume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.MinCpuPlatform != nil {
		in, out := &in.MinCpuPlatform, &out.MinCpuPlatform
		*out = new(string)
		**out = **in
	}
	if in.ServiceAccount != nil {
		in, out := &in.ServiceAccount, &out.ServiceAccount
		*out = new(ServiceAccount)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeTemplate != nil {
		in, out := &in.NodeTemplate, &out.NodeTemplate
		*out = new(v1alpha1.NodeTemplate)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerConfig.
func (in *WorkerConfig) DeepCopy() *WorkerConfig {
	if in == nil {
		return nil
	}
	out := new(WorkerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerStatus) DeepCopyInto(out *WorkerStatus) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.MachineImages != nil {
		in, out := &in.MachineImages, &out.MachineImages
		*out = make([]MachineImage, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerStatus.
func (in *WorkerStatus) DeepCopy() *WorkerStatus {
	if in == nil {
		return nil
	}
	out := new(WorkerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerStatus) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadIdentityConfig) DeepCopyInto(out *WorkloadIdentityConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.CredentialsConfig != nil {
		in, out := &in.CredentialsConfig, &out.CredentialsConfig
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentityConfig.
func (in *WorkloadIdentityConfig) DeepCopy() *WorkloadIdentityConfig {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentityConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkloadIdentityConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
