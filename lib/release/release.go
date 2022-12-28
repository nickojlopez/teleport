/*
Copyright 2022 Gravitational, Inc.

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
package release

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ClientConfig contains configuration for the release client
type ClientConfig struct {
	// TLSConfig is the client TLS configuration
	TLSConfig *tls.Config
}

// CheckAndSetDefaults checks and sets default config values
func (c *ClientConfig) CheckAndSetDefaults() error {
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLS configuration")
	}

	return nil
}

// Client is a client to make HTTPS requests to the
// release server using the Teleport Enterprise license
// as authentication
type Client struct {
	// client is the client used to make calls to the release API
	client *roundtrip.Client
}

// NewClient returns a new release client with a client
// to make https requests to the release server
func NewClient(cfg ClientConfig) (*Client, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: cfg.TLSConfig,
		},
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := roundtrip.NewClient(fmt.Sprintf("https://%s", cfg.TLSConfig.ServerName), "", roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		client: client,
	}, nil
}

// ListRelease calls the release server and returns a list of releases
func (c *Client) ListReleases(ctx context.Context) ([]*types.Release, error) {
	if c.client == nil {
		return nil, trace.BadParameter("client not initialized")
	}

	resp, err := c.client.Get(ctx, c.client.Endpoint("teleport-ent"), nil)
	if err != nil {
		log.WithError(err).Error()
		return nil, trace.Wrap(err)
	}

	if resp.Code() == http.StatusUnauthorized {
		return nil, trace.AccessDenied("access denied by the release server")
	}

	var releases []Release
	err = json.Unmarshal(resp.Bytes(), &releases)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	responseReleases := []*types.Release{}
	for _, r := range releases {
		releaseAssets := []*types.Asset{}
		for _, a := range r.Assets {
			releaseAssets = append(releaseAssets, &types.Asset{
				Arch:        a.Arch,
				Description: a.Description,
				Name:        a.Name,
				OS:          a.OS,
				SHA256:      a.SHA256,
				Size_:       a.Size,
				DisplaySize: byteCount(a.Size),
				ReleaseIDs:  a.ReleaseIDs,
				PublicURL:   a.PublicURL,
			})
		}

		responseReleases = append(responseReleases, &types.Release{
			NotesMD:   r.NotesMD,
			Product:   r.Product,
			ReleaseID: r.ReleaseID,
			Status:    r.Status,
			Version:   r.Version,
			Assets:    releaseAssets,
		})
	}

	return responseReleases, err
}

// byteCount converts a size in bytes to a human-readable string.
func byteCount(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

// GetServerAddr returns the release server address from the environment
// variables or, if not set, the default value
func GetServerAddr() string {
	addr := os.Getenv(types.ReleaseServerEnvVar)
	if addr == "" {
		addr = types.DefaultReleaseServerAddr
	}
	return addr
}

// Release correspond to a Teleport Enterprise release
// returned by the release service
type Release struct {
	// NotesMD is the notes of the release in markdown
	NotesMD string `json:"notesMd"`
	// Product is the release product, teleport or teleport-ent
	Product string `json:"product"`
	// ReleaseId is the ID of the product
	ReleaseID string `json:"releaseIds"`
	// Status is the status of the release
	Status string `json:"status"`
	// Version is the version of the release
	Version string `json:"version"`
	// Asset is a list of assets related to the release
	Assets []*Asset `json:"assets"`
}

// Asset represents a release asset returned by the
// release service
type Asset struct {
	// Arch is the architecture of the asset
	Arch string `json:"arch"`
	// Description is the description of the asset
	Description string `json:"description"`
	// Name is the name of the asset
	Name string `json:"name"`
	// Name is which OS the asset is built for
	OS string `json:"os"`
	// SHA256 is the sha256 of the asset
	SHA256 string `json:"sha256"`
	// Size is the size of the release in bytes
	Size int64 `json:"size"`
	// ReleaseID is a list of releases that have the asset included
	ReleaseIDs []string `json:"releaseIds"`
	// PublicURL is the public URL used to download the asset
	PublicURL string `json:"publicUrl"`
}
