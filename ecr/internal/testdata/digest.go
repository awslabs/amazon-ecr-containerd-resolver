package testdata

import "github.com/opencontainers/go-digest"

const (
	// InsignificantDigest is an arbitrary value for consistent, placeholder use
	// cases in tests.
	InsignificantDigest digest.Digest = "insignificant-digest"
	// LayerDigest is used for consistent, placeholder layer digests in tests.
	LayerDigest digest.Digest = "layer-digest"
	// ImageDigest is used for consistent, placeholder image digests in tests.
	ImageDigest digest.Digest = "image-digest"
)
