// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

// UpdateMachineImagesStatus updates the machine image status
// with the used machine images for the `Worker` resource.
func (w *WorkerDelegate) UpdateMachineImagesStatus(ctx context.Context) error {
	if w.machineImages == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return fmt.Errorf("unable to generate the machine config: %w", err)
		}
	}

	// Decode the current worker provider status.
	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return fmt.Errorf("unable to decode the worker provider status: %w", err)
	}

	workerStatus.MachineImages = w.machineImages
	if err := w.updateWorkerProviderStatus(ctx, workerStatus); err != nil {
		return fmt.Errorf("unable to update worker provider status: %w", err)
	}

	return nil
}

func (w *WorkerDelegate) selectMachineImageForWorkerPool(name, version string, workerArchitecture *string, machineCapabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) (*apisgcp.MachineImage, error) {
	selectedMachineImage := &apisgcp.MachineImage{
		Name:         name,
		Version:      version,
		Architecture: workerArchitecture,
	}
	if capabilitySet, err := helper.FindImageInCloudProfile(w.cloudProfileConfig, name, version, workerArchitecture, machineCapabilities, capabilityDefinitions); err == nil {
		selectedMachineImage.Capabilities = capabilitySet.Capabilities
		selectedMachineImage.Image = capabilitySet.Image
		return selectedMachineImage, nil
	}
	// Try to look up machine image in worker provider status as it was not found in componentconfig.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &apisgcp.WorkerStatus{}
		if _, _, err := w.decoder.Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, fmt.Errorf("could not decode worker status of worker '%s': %w", k8sclient.ObjectKeyFromObject(w.worker), err)
		}

		return helper.FindImageInWorkerStatus(workerStatus.MachineImages, name, version, machineCapabilities, capabilityDefinitions)
	}

	return nil, worker.ErrorMachineImageNotFound(name, version, *workerArchitecture)
}

// appendMachineImage appends a machine image to the list if it doesn't already exist with the same capabilities or architecture.
// Note: capabilityDefinitions is expected to be original
func appendMachineImage(machineImages []apisgcp.MachineImage, machineImage apisgcp.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []apisgcp.MachineImage {
	// support for cloudprofile machine images without capabilities
	if len(capabilityDefinitions) == 0 {
		for _, image := range machineImages {
			isArchEqual := ptr.Deref(image.Architecture, v1beta1constants.ArchitectureAMD64) == ptr.Deref(machineImage.Architecture, v1beta1constants.ArchitectureAMD64)
			if image.Name == machineImage.Name && image.Version == machineImage.Version && isArchEqual {
				// If the image already exists without capabilities, we can just return the existing list.
				return machineImages
			}
		}
		return append(machineImages, apisgcp.MachineImage{
			Name:         machineImage.Name,
			Version:      machineImage.Version,
			Image:        machineImage.Image,
			Architecture: machineImage.Architecture,
		})
	}

	defaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(machineImage.Capabilities, capabilityDefinitions)

	for _, existingMachineImage := range machineImages {
		existingDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(existingMachineImage.Capabilities, capabilityDefinitions)
		if existingMachineImage.Name == machineImage.Name && existingMachineImage.Version == machineImage.Version && gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilities, existingDefaultedCapabilities) {
			// If the image already exists with the same capabilities return the existing list.
			return machineImages
		}
	}

	// If the image does not exist, we create a new machine image entry with the capabilities.
	return append(machineImages, apisgcp.MachineImage{
		Name:         machineImage.Name,
		Version:      machineImage.Version,
		Image:        machineImage.Image,
		Capabilities: machineImage.Capabilities,
	})
}
