package ecr

import (
	"context"
	"testing"

	"github.com/containerd/containerd/images"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestParseImageManifestMediaType(t *testing.T) {
	cases := []struct {
		name      string
		manifest  string
		mediaType string
	}{
		{
			name:      "default",
			manifest:  "",
			mediaType: images.MediaTypeDockerSchema2Manifest,
		},
		{
			name:      "schemaVersion:1 unsigned",
			manifest:  `{"schemaVersion": 1}`,
			mediaType: "application/vnd.docker.distribution.manifest.v1+json",
		},
		{
			name:      "schemaVersion:1",
			manifest:  `{"schemaVersion": 1, "signatures":[{}]}`,
			mediaType: images.MediaTypeDockerSchema1Manifest,
		},
		{
			name:      "schemaVersion:2 docker",
			manifest:  `{"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.v2+json"}`,
			mediaType: images.MediaTypeDockerSchema2Manifest,
		},
		{
			name:      "schemaVersion:2 oci",
			manifest:  `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json"}`,
			mediaType: ocispec.MediaTypeImageManifest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mediaType := parseImageManifestMediaType(context.TODO(), tc.manifest)
			assert.Equal(t, tc.mediaType, mediaType)
		})
	}
}
