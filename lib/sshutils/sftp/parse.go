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
	"strings"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// Destination is a SFTP destination to copy to or from
type Destination struct {
	// Login is an optional login username
	Login string
	// Host is a host to copy to/from
	Host *utils.NetAddr
	// Path is a path to copy to/from.
	// An empty path name is valid, and it refers to the user's default directory (usually
	// the user's home directory).
	// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
	Path string
}

// ParseDestination takes a string representing a remote resource for SFTP
// to download/upload in the form "[user@]host:[path]" and parses it into
// a structured form.
//
// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
// section about details on file names.
func ParseDestination(input string) (*Destination, error) {
	firstColonIdx := strings.Index(input, ":")
	if firstColonIdx == -1 {
		return nil, trace.BadParameter("%q is missing a path, use form [user@]host:[path]", input)
	}
	loginEndIdx := strings.LastIndex(input, "@")
	// if there is no input after the last '@', then host and path are missing
	if len(input) == loginEndIdx+1 {
		return nil, trace.BadParameter("%q is missing a host and path, use form [user@]host:[path]", input)
	}

	var login string
	// If at least one '@' exists and is before the first ':', get the
	// login. Otherwise, either there are no '@' or all '@' are after
	// the first ':' (where the host or path starts), so no login was
	// specified.
	if loginEndIdx != -1 && loginEndIdx < firstColonIdx {
		login = input[:loginEndIdx]
		loginEndIdx++
	} else {
		loginEndIdx = 0
	}

	hostEnd := firstColonIdx + 1
	host, err := utils.ParseAddr(input[loginEndIdx:firstColonIdx])
	if err != nil {
		// parsing the input after the login failed, so the host is
		// invalid unless the host is an IPv6 address, as then the
		// host won't end at the first colon

		// if there is only one ':' in the entire input, the host isn't
		// an IPv6 address
		if strings.Count(input, ":") == 1 {
			return nil, trace.Wrap(err)
		}

		// an IPv6 address here must be enclosed by braces, if the host
		// isn't this isn't an IPv6 address
		afterLogin := input[loginEndIdx:]
		if afterLogin[0] != '[' {
			return nil, trace.Wrap(err)
		}
		rbraceIdx := strings.Index(afterLogin, "]")
		if rbraceIdx == -1 {
			return nil, trace.Wrap(err)
		}
		// if there's nothing after ']' then the path is missing
		if len(afterLogin) <= rbraceIdx+2 {
			return nil, trace.Wrap(err)
		}

		maybeAddr := afterLogin[:rbraceIdx+1]
		host, err = utils.ParseAddr(maybeAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// the host ends after the login + the IPv6 address
		// (including the trailing ']') and a ':'
		hostEnd = loginEndIdx + rbraceIdx + 2
	}

	// if there is nothing after the host the path defaults to "."
	path := "."
	if len(input) > hostEnd {
		path = input[hostEnd:]
	}

	return &Destination{
		Login: login,
		Host:  host,
		Path:  path,
	}, nil
}
