package apimodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatchPreferencesRequest_Validate(t *testing.T) {
	t.Parallel()

	validBase := PatchPreferencesRequest{
		DefaultPrivacy:     "public",
		DefaultQuotePolicy: "public",
	}

	t.Run("valid BCP-47 language tags", func(t *testing.T) {
		t.Parallel()

		cases := []string{"en", "en-US", "zh-Hans", "zh-Hans-CN", "pt-BR", "sr-Latn-RS"}
		for _, lang := range cases {
			req := validBase
			req.DefaultLanguage = lang
			assert.NoError(t, req.Validate(), "expected %q to be valid", lang)
		}
	})

	t.Run("empty language is valid (clears preference)", func(t *testing.T) {
		t.Parallel()
		req := validBase
		req.DefaultLanguage = ""
		assert.NoError(t, req.Validate())
	})

	t.Run("invalid language tags", func(t *testing.T) {
		t.Parallel()

		cases := []string{"english", "e", "123", "en_US", "toolongprimary"}
		for _, lang := range cases {
			req := validBase
			req.DefaultLanguage = lang
			assert.Error(t, req.Validate(), "expected %q to be invalid", lang)
		}
	})
}
