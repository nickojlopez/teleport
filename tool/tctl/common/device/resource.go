// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package device

import (
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/utils"
)

type Resource types.DeviceV1

// checkAndSetDefaults sanity checks Resource fields to catch simple errors, and
// sets default values for all fields with defaults.
func (r *Resource) checkAndSetDefaults() error {
	// Assign defaults:
	// - Kind = device
	// - Metadata.Name = UUID
	// - Spec.EnrollStatus = unspecified
	if r.Kind == "" {
		r.Kind = types.KindDevice
	}
	if r.Metadata.Name == "" {
		r.Metadata.Name = uuid.NewString()
	}
	if r.Spec.EnrollStatus == "" {
		r.Spec.EnrollStatus =
			devicetrust.ResourceEnrollStatusToString(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED)
	}

	// Validate Metadata.
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Validate "simple" fields.
	switch {
	case r.Kind != types.KindDevice: // Sanity check.
		return trace.BadParameter("unexpected resource kind %q, must be %q", r.Kind, types.KindDevice)
	case r.Spec.OsType == "":
		return trace.BadParameter("missing OS type")
	case r.Spec.AssetTag == "":
		return trace.BadParameter("missing asset tag")
	}

	// Validate enum conversions.
	if _, err := devicetrust.ResourceOSTypeFromString(r.Spec.OsType); err != nil {
		return trace.Wrap(err)
	}
	if _, err := devicetrust.ResourceEnrollStatusFromString(r.Spec.EnrollStatus); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UnmarshalDevice parses a device in the Resource format which matches
// the expected YAML format for Teleport resources, sets default values, and
// converts to *devicepb.Device.
func UnmarshalDevice(raw []byte) (*devicepb.Device, error) {
	var resource Resource
	if err := utils.FastUnmarshal(raw, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := resource.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resourceToProto(&resource)
}

// ProtoToResource converts a *devicepb.Device into a *Resource which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ProtoToResource(dev *devicepb.Device) *Resource {
	toTimePtr := func(pb *timestamppb.Timestamp) *time.Time {
		if pb == nil {
			return nil
		}
		t := pb.AsTime()
		return &t
	}

	var cred *types.DeviceCredential
	if dev.Credential != nil {
		cred = &types.DeviceCredential{
			Id:           dev.Credential.Id,
			PublicKeyDer: dev.Credential.PublicKeyDer,
		}
	}

	collectedData := make([]*types.DeviceCollectedData, len(dev.CollectedData))
	for i, d := range dev.CollectedData {
		collectedData[i] = &types.DeviceCollectedData{
			CollectTime:  toTimePtr(d.CollectTime),
			RecordTime:   toTimePtr(d.RecordTime),
			OsType:       devicetrust.ResourceOSTypeToString(d.OsType),
			SerialNumber: d.SerialNumber,
		}
	}

	return &Resource{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindDevice,
			Version: dev.ApiVersion,
			Metadata: types.Metadata{
				Name: dev.Id,
			},
		},
		Spec: &types.DeviceSpec{
			OsType:        devicetrust.ResourceOSTypeToString(dev.OsType),
			AssetTag:      dev.AssetTag,
			CreateTime:    toTimePtr(dev.CreateTime),
			UpdateTime:    toTimePtr(dev.UpdateTime),
			EnrollStatus:  devicetrust.ResourceEnrollStatusToString(dev.EnrollStatus),
			Credential:    cred,
			CollectedData: collectedData,
		},
	}
}

func resourceToProto(r *Resource) (*devicepb.Device, error) {
	toTimePB := func(t *time.Time) *timestamppb.Timestamp {
		if t == nil {
			return nil
		}
		return timestamppb.New(*t)
	}

	osType, err := devicetrust.ResourceOSTypeFromString(r.Spec.OsType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	enrollStatus, err := devicetrust.ResourceEnrollStatusFromString(r.Spec.EnrollStatus)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cred *devicepb.DeviceCredential
	if r.Spec.Credential != nil {
		cred = &devicepb.DeviceCredential{
			Id:           r.Spec.Credential.Id,
			PublicKeyDer: r.Spec.Credential.PublicKeyDer,
		}
	}

	collectedData := make([]*devicepb.DeviceCollectedData, len(r.Spec.CollectedData))
	for i, d := range r.Spec.CollectedData {
		dataOSType, err := devicetrust.ResourceOSTypeFromString(d.OsType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		collectedData[i] = &devicepb.DeviceCollectedData{
			CollectTime:  toTimePB(d.CollectTime),
			RecordTime:   toTimePB(d.RecordTime),
			OsType:       dataOSType,
			SerialNumber: d.SerialNumber,
		}
	}

	return &devicepb.Device{
		ApiVersion:    r.Version,
		Id:            r.Metadata.Name,
		OsType:        osType,
		AssetTag:      r.Spec.AssetTag,
		CreateTime:    toTimePB(r.Spec.CreateTime),
		UpdateTime:    toTimePB(r.Spec.UpdateTime),
		EnrollStatus:  enrollStatus,
		Credential:    cred,
		CollectedData: collectedData,
	}, nil
}
