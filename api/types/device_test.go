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

package types

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// TestUnmarshalDevice tests that devices can be successfully
// unmarshalled from YAML and JSON.
func TestUnmarshalDevice(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		input         string
		errorContains string
		expected      *DeviceV1
	}{
		{
			desc: "success",
			input: `
{
  "kind": "device",
	"version": "v1",
	"metadata": {
		"name": "xaa"
	},
	"spec": {
		"asset_tag": "mymachine",
		"os_type": "macos",
		"enroll_status": "enrolled"
	}
}`,
			expected: &DeviceV1{
				ResourceHeader: ResourceHeader{
					Kind:    KindDevice,
					Version: "v1",
					Metadata: Metadata{
						Namespace: defaults.Namespace,
						Name:      "xaa",
					},
				},
				Spec: &DeviceSpec{
					OsType:       "macos",
					AssetTag:     "mymachine",
					EnrollStatus: "enrolled",
				},
			},
		},
		{
			desc:          "fail string as num",
			errorContains: `cannot unmarshal number`,
			input: `
{
  "kind": "device",
	"version": "v1",
	"metadata": {
		"name": "secretid"
	},
	"spec": {
		"asset_tag": 4,
		"os_type": "macos",
		"enroll_status": "enrolled"
	}
}`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			out, err := UnmarshalDevice([]byte(tc.input))
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error from UnmarshalDevice does not contain the expected string")
				return
			}
			require.NoError(t, err, "UnmarshalDevice returned unexpected error")
			require.Equal(t, tc.expected, out, "unmarshalled device  does not match what was expected")
		})
	}
}

func TestDeviceConversions_toAndFrom(t *testing.T) {
	t1 := time.UnixMilli(1680276526972000) // Fri Mar 31 2023 15:28:46 UTC
	t11 := t1.Add(100 * time.Millisecond)
	t2 := t1.Add(1 * time.Second)
	t22 := t1.Add(100 * time.Millisecond)

	const osType = devicepb.OSType_OS_TYPE_MACOS
	const assetTag = "llama14"
	dev := &devicepb.Device{
		ApiVersion:   "v1",
		Id:           "0af7c335-5f2c-4756-8266-9965a47ccbd3",
		OsType:       osType,
		AssetTag:     assetTag,
		CreateTime:   timestamppb.New(t1),
		UpdateTime:   timestamppb.New(t2),
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
		Credential: &devicepb.DeviceCredential{
			Id:           "557762f0-4cd4-4b75-aaee-575c57237c0b",
			PublicKeyDer: []byte("insert public key here"),
		},
		CollectedData: []*devicepb.DeviceCollectedData{
			{
				CollectTime:  timestamppb.New(t1),
				RecordTime:   timestamppb.New(t11),
				OsType:       osType,
				SerialNumber: assetTag,
			},
			{
				CollectTime:  timestamppb.New(t2),
				RecordTime:   timestamppb.New(t22),
				OsType:       osType,
				SerialNumber: assetTag,
			},
		},
	}

	gotRes := DeviceToResource(dev)
	// Assert some of the more "unusual" or missing fields.
	// We know other information isn't lost because of the conversion below,
	// therefore it must be present in the resource.
	assert.Equal(t, dev.ApiVersion, gotRes.Version, "resource.Version is not the ApiVersion")
	assert.Equal(t, dev.Id, gotRes.Metadata.Name, "resource.Metadata.Name is not the Id")
	assert.NotEmpty(t, gotRes.Metadata.Namespace, "resource.Metadata.Namespace")

	gotDev, err := DeviceFromResource(gotRes)
	require.NoError(t, err, "DeviceFromResource failed")
	if diff := cmp.Diff(dev, gotDev, protocmp.Transform()); diff != "" {
		t.Errorf("DeviceFromResource mismatch (-want +got)\n%s", diff)
	}
}
