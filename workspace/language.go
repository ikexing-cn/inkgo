package workspace

import "fmt"

// Language is the language of a book, independent of the interface.
type Language string

const (
	// Chinese is Simplified Chinese.
	Chinese Language = "zh-Hans"
	// English is English.
	English Language = "en"
)

// LanguageError reports an unsupported book language in work.yaml.
type LanguageError struct{ Value string }

// Error describes the invalid value for diagnostics.
func (e *LanguageError) Error() string { return fmt.Sprintf("language: unsupported %q", e.Value) }

// OtherCategory is the book's catch-all category name.
func (l Language) OtherCategory() string {
	if l == English {
		return "Other"
	}
	return Other
}

// Unit identifies the book's counting unit for localized messages.
func (l Language) Unit() string {
	if l == English {
		return "word"
	}
	return "character"
}

// ASCIICharsPerToken is the approximate ASCII characters per token.
// English follows OpenAI's four-character rule of thumb; Chinese keeps
// the existing three-character margin for embedded Latin text.
func (l Language) ASCIICharsPerToken() int {
	if l == English {
		return 4
	}
	return 3
}
