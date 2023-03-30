/*
Copyright 2023 Gravitational, Inc.

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

package sftp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

var testCases = []struct {
	comment string
	in      string
	dest    Destination
	err     string
}{
	{
		comment: "full spec of the remote destination",
		in:      "root@remote.host:/etc/nginx.conf",
		dest: Destination{
			Login: "root",
			Host: &utils.NetAddr{
				Addr:        "remote.host",
				AddrNetwork: "tcp",
			},
			Path: "/etc/nginx.conf",
		},
	},
	{
		comment: "spec with just the remote host",
		in:      "remote.host:/etc/nginx.co:nf",
		dest: Destination{
			Host: &utils.NetAddr{
				Addr:        "remote.host",
				AddrNetwork: "tcp",
			},
			Path: "/etc/nginx.co:nf",
		},
	},
	{
		comment: "ipv6 remote destination address",
		in:      "[::1]:/etc/nginx.co:nf",
		dest: Destination{
			Host: &utils.NetAddr{
				Addr:        "[::1]",
				AddrNetwork: "tcp",
			},
			Path: "/etc/nginx.co:nf",
		},
	},
	{
		comment: "full spec of the remote destination using ipv4 address",
		in:      "root@123.123.123.123:/var/www/html/",
		dest: Destination{
			Login: "root",
			Host: &utils.NetAddr{
				Addr:        "123.123.123.123",
				AddrNetwork: "tcp",
			},
			Path: "/var/www/html/",
		},
	},
	{
		comment: "target location using wildcard",
		in:      "myusername@myremotehost.com:/home/hope/*",
		dest: Destination{
			Login: "myusername",
			Host: &utils.NetAddr{
				Addr:        "myremotehost.com",
				AddrNetwork: "tcp",
			},
			Path: "/home/hope/*",
		},
	},
	{
		comment: "complex login",
		in:      "complex@example.com@remote.com:/anything.txt",
		dest: Destination{
			Login: "complex@example.com",
			Host: &utils.NetAddr{
				Addr:        "remote.com",
				AddrNetwork: "tcp",
			},
			Path: "/anything.txt",
		},
	},
	{
		comment: "implicit user's home directory",
		in:      "root@remote.host:",
		dest: Destination{
			Login: "root",
			Host: &utils.NetAddr{
				Addr:        "remote.host",
				AddrNetwork: "tcp",
			},
			Path: ".",
		},
	},
	{
		comment: "no login and '@' in path",
		in:      "remote.host:/some@file",
		dest: Destination{
			Host: &utils.NetAddr{
				Addr:        "remote.host",
				AddrNetwork: "tcp",
			},
			Path: "/some@file",
		},
	},
	{
		comment: "no login, '@' and ':' in path",
		in:      "remote.host:/some@remote:file",
		dest: Destination{
			Host: &utils.NetAddr{
				Addr:        "remote.host",
				AddrNetwork: "tcp",
			},
			Path: "/some@remote:file",
		},
	},
	{
		comment: "complex login, IPv6 addr and ':' in path",
		in:      "complex@user@[::1]:/remote:file",
		dest: Destination{
			Login: "complex@user",
			Host: &utils.NetAddr{
				Addr:        "[::1]",
				AddrNetwork: "tcp",
			},
			Path: "/remote:file",
		},
	},
}

func TestParseDestination(t *testing.T) {
	t.Parallel()

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.comment, func(t *testing.T) {
			resp, err := ParseDestination(tt.in)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(resp, &tt.dest))
		})
	}
}

func FuzzParseDestination(f *testing.F) {
	for _, tt := range testCases {
		f.Add(tt.in)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = ParseDestination(input)
	})
}
