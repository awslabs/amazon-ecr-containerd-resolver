package testdata

import "strings"

// MediaTypeSample provides a sample document for a given mediaType.
type MediaTypeSample struct {
	mediaType string
	content string
}

// MediaType is the defined sample's actual mediaType.
func (s *MediaTypeSample) MediaType() string {
	return s.mediaType
}

// Content provides the sample's JSON data as a string.
func (s *MediaTypeSample) Content() string {
	return strings.TrimSpace(s.content)
}

// EmptySample is an edge case sample, use
var EmptySample = MediaTypeSample{
	mediaType: "",
	content: `
{
  "updog": "whats"
}`,
}
