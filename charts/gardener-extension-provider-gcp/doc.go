// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate sh -c "bash $GARDENER_HACK_DIR/generate-controller-registration.sh provider-gcp . $(cat ../../VERSION) ../../example/controller-registration.yaml BackupBucket:gcp BackupEntry:gcp Bastion:gcp ControlPlane:gcp DNSRecord:google-clouddns Infrastructure:gcp Worker:gcp"

// Package chart enables go:generate support for generating the correct controller registration.
package chart
