package model

import (
	"errors"
	"fmt"
)

// ErrRequestTooLarge reports request exceeds 8 MiB.
var ErrRequestTooLarge = errors.New("model: request exceeds 8 MiB")

// ErrTruncated reports reply truncated.
var ErrTruncated = errors.New("model: reply truncated")

// ErrKeyNewline reports api key contains a newline.
var ErrKeyNewline = errors.New("model: api key contains a newline")

// ErrRedirect reports redirects disabled.
var ErrRedirect = errors.New("model: redirects disabled")

// ErrModelRequired reports model id required.
var ErrModelRequired = errors.New("model: model id required")

// ErrNegativeTokens reports max tokens is negative.
var ErrNegativeTokens = errors.New("model: max tokens is negative")

// ErrMessagesRequired reports at least one message required.
var ErrMessagesRequired = errors.New("model: at least one message required")

// ErrToolJSON reports tool arguments are not JSON.
var ErrToolJSON = errors.New("model: tool arguments are not JSON")

// ErrDisconnected reports reply disconnected before completion.
var ErrDisconnected = errors.New("model: reply disconnected before completion")

// ErrFiltered reports reply filtered.
var ErrFiltered = errors.New("model: reply filtered")

// ErrIncomplete reports reply incomplete.
var ErrIncomplete = errors.New("model: reply incomplete")

// ErrReplyTooLarge reports reply exceeds 8 MiB.
var ErrReplyTooLarge = errors.New("model: reply exceeds 8 MiB")

// ErrNoText reports service returned no text.
var ErrNoText = errors.New("model: service returned no text")

// ErrToolCallMissing reports tool result has no matching call.
var ErrToolCallMissing = errors.New("model: tool result has no matching call")

// ErrRole reports invalid message role; use Request.System for instructions.
var ErrRole = errors.New("model: invalid message role; use Request.System for instructions")

// ErrService reports service returned an error; details redacted.
var ErrService = errors.New("model: service returned an error; details redacted")

// ErrConnection reports cannot connect to service.
var ErrConnection = errors.New("model: cannot connect to service")

// ErrDestination reports request destination differs from configuration.
var ErrDestination = errors.New("model: request destination differs from configuration")

// ErrConfigTooLarge reports configuration exceeds 1 MiB.
var ErrConfigTooLarge = errors.New("model: configuration exceeds 1 MiB")

// ErrConfigYAML reports invalid configuration YAML.
var ErrConfigYAML = errors.New("model: invalid configuration YAML")

// ErrConfigDocuments reports configuration contains multiple YAML documents.
var ErrConfigDocuments = errors.New("model: configuration contains multiple YAML documents")

// ErrConfigEncode reports cannot encode configuration.
var ErrConfigEncode = errors.New("model: cannot encode configuration")

// ErrIDFormat reports invalid provider id.
var ErrIDFormat = errors.New("model: invalid provider id")

// ErrConfigVersion reports unsupported configuration version; expected 1.
var ErrConfigVersion = errors.New("model: unsupported configuration version; expected 1")

// ErrProviderDuplicate reports duplicate provider id.
var ErrProviderDuplicate = errors.New("model: duplicate provider id")

// ErrNameRequired reports display name required.
var ErrNameRequired = errors.New("model: display name required")

// ErrProtocol reports protocol must be chat, responses or anthropic.
var ErrProtocol = errors.New("model: protocol must be chat, responses or anthropic")

// ErrEnvFormat reports invalid environment variable name.
var ErrEnvFormat = errors.New("model: invalid environment variable name")

// ErrURLScheme reports api url must start with http:// or https://.
var ErrURLScheme = errors.New("model: api url must start with http:// or https://")

// ErrURLHost reports api url has no host.
var ErrURLHost = errors.New("model: api url has no host")

// ErrURLExtra reports api url contains credentials, query or fragment.
var ErrURLExtra = errors.New("model: api url contains credentials, query or fragment")

// ErrModelID reports model id is empty or has surrounding whitespace.
var ErrModelID = errors.New("model: model id is empty or has surrounding whitespace")

// ErrCatalogTotal reports catalog total exceeds 8 MiB.
var ErrCatalogTotal = errors.New("model: catalog total exceeds 8 MiB")

// ErrCatalogJSON reports catalog is not valid JSON.
var ErrCatalogJSON = errors.New("model: catalog is not valid JSON")

// ErrCatalogService reports catalog service returned an error.
var ErrCatalogService = errors.New("model: catalog service returned an error")

// ErrCatalogData reports catalog has no data field.
var ErrCatalogData = errors.New("model: catalog has no data field")

// ErrCatalogPagination reports invalid or unsupported catalog pagination.
var ErrCatalogPagination = errors.New("model: invalid or unsupported catalog pagination")

// ErrCatalogPages reports catalog exceeds 100 pages.
var ErrCatalogPages = errors.New("model: catalog exceeds 100 pages")

// ErrRequestCreate reports cannot create request.
var ErrRequestCreate = errors.New("model: cannot create request")

// ErrCatalogPageSize reports catalog page exceeds 8 MiB.
var ErrCatalogPageSize = errors.New("model: catalog page exceeds 8 MiB")

// ErrKeyDelete reports keychain cannot delete api key.
var ErrKeyDelete = errors.New("model: keychain cannot delete api key")

// ErrKeySave reports keychain cannot save api key; plaintext fallback disabled.
var ErrKeySave = errors.New("model: keychain cannot save api key; plaintext fallback disabled")

// ErrKeychain reports keychain unavailable.
var ErrKeychain = errors.New("model: keychain unavailable")

// ErrModelFields reports unsupported model configuration fields.
var ErrModelFields = errors.New("model: unsupported model fields")

// LimitsError identifies a model and the invalid limit rule.
type LimitsError struct {
	ID   string
	Rule string
}

func (e *LimitsError) Error() string {
	return fmt.Sprintf("model: invalid %s for model %q", e.Rule, e.ID)
}

// ContextLimitError identifies the part that exceeds the context window.
type ContextLimitError struct{ Part string }

func (e *ContextLimitError) Error() string { return "model: context limit exceeded by " + e.Part }

// ConfigError identifies a configuration IO operation and its cause.
type ConfigError struct {
	Op  string
	Err error
}

func (e *ConfigError) Error() string { return fmt.Sprintf("model: %s configuration: %v", e.Op, e.Err) }

func (e *ConfigError) Unwrap() error { return e.Err }

// ProviderError identifies the provider position whose configuration failed.
type ProviderError struct {
	Count int
	Err   error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("model: provider %d: %v", e.Count, e.Err)
}

func (e *ProviderError) Unwrap() error { return e.Err }

// ProviderMissingError identifies a provider not configured on this computer.
type ProviderMissingError struct{ ID string }

func (e *ProviderMissingError) Error() string {
	return fmt.Sprintf("model: provider %q not configured", e.ID)
}

// EffortError identifies an unsupported reasoning effort.
type EffortError struct{ Effort string }

func (e *EffortError) Error() string {
	return fmt.Sprintf("model: unsupported reasoning effort %q", e.Effort)
}

// ToolError identifies a tool with an invalid name or schema.
type ToolError struct{ Name string }

func (e *ToolError) Error() string {
	return fmt.Sprintf("model: tool %q has no name or JSON schema", e.Name)
}

// TimeoutError wraps the cause of a model request timeout.
type TimeoutError struct{ Err error }

func (e *TimeoutError) Error() string { return fmt.Sprintf("model: request timed out: %v", e.Err) }

func (e *TimeoutError) Unwrap() error { return e.Err }

// AbortedError wraps the cause of a canceled model request.
type AbortedError struct{ Err error }

func (e *AbortedError) Error() string { return fmt.Sprintf("model: request aborted: %v", e.Err) }

func (e *AbortedError) Unwrap() error { return e.Err }

// HTTPError reports a status without leaking the response body.
type HTTPError struct{ Code int }

func (e *HTTPError) Error() string { return fmt.Sprintf("model: service returned HTTP %d", e.Code) }
