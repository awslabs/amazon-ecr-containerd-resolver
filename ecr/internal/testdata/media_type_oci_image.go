package testdata

import (
  ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var OCIImageManifest = MediaTypeSample{
	mediaType: ocispec.MediaTypeImageManifest,
	content: `
{
  "schemaVersion": 2,
  "config": {
    "mediaType": "application/vnd.oci.image.config.v1+json",
    "digest": "sha256:a6ff6fb34ad5a20c2b2371013918a9f0e033a77460b2f17a4041e02bd3d252d0",
    "size": 302
  },
  "layers": [
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "digest": "sha256:55e3debf4607c47ff150940897a656ec79859f7aa715f26ab4357065e2e20535",
      "size": 62599745
    }
  ]
}
`,
}

var OCIImageIndex = MediaTypeSample{
	mediaType: ocispec.MediaTypeImageIndex,
	content: `
{
  "schemaVersion": 2,
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 3231,
      "digest": "sha256:babb154b919b9ad7d38786f71f9c8a3614f6d017b0ba7cada4801ceed7b2220d",
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 3231,
      "digest": "sha256:718441d735e6a7c9b24837c779cc7112995289eff976a308695a1936bc20b67b",
      "platform": {
        "architecture": "arm64",
        "os": "linux"
      }
    }
  ]
}
`,
}
