package testdata

import (
	"encoding/json"
	"strings"
)

type MediaTypeSample interface {
	MediaType() string
	Content() string
}

// MediaTypeSample provides a sample document for a given mediaType.
type mediaTypeSample struct {
	mediaType string
	content string
}

// MediaType is the defined sample's actual mediaType.
func (s *mediaTypeSample) MediaType() string {
	return s.mediaType
}

// Content provides the sample's JSON data as a string.
func (s *mediaTypeSample) Content() string {
	return strings.TrimSpace(s.content)
}

// EmptySample is an edge case sample, use
var EmptySample MediaTypeSample = &mediaTypeSample{
	mediaType: "",
	content: `{}`,
}

func WithMediaTypeRemoved(src MediaTypeSample) MediaTypeSample {
	m := map[string]interface{}{}
	err := json.Unmarshal([]byte(src.Content()), &m)
	if err != nil {
		return src
	}
	if _, ok := m["mediaType"]; ok {
		return src
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		panic(err)
	}
	return &mediaTypeSample{
		mediaType: src.MediaType(),
		content: string(data),
	}
}
