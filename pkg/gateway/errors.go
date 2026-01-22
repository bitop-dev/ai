package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

// GatewayErrorType identifies gateway error classifications.
type GatewayErrorType string

const (
	GatewayErrorTypeAuthentication GatewayErrorType = "authentication_error"
	GatewayErrorTypeInvalidRequest GatewayErrorType = "invalid_request_error"
	GatewayErrorTypeRateLimit      GatewayErrorType = "rate_limit_exceeded"
	GatewayErrorTypeModelNotFound  GatewayErrorType = "model_not_found"
	GatewayErrorTypeInternalServer GatewayErrorType = "internal_server_error"
	GatewayErrorTypeResponseError  GatewayErrorType = "response_error"
)

// GatewayAuthMethod identifies how the gateway was authenticated.
type GatewayAuthMethod string

const (
	GatewayAuthMethodAPIKey GatewayAuthMethod = "api-key"
	GatewayAuthMethodOIDC   GatewayAuthMethod = "oidc"
)

// GatewayError is the base gateway error.
type GatewayError struct {
	Type       GatewayErrorType
	Message    string
	StatusCode int
	Cause      error
}

func (err *GatewayError) Error() string {
	if err == nil {
		return ""
	}
	if err.Message != "" {
		return err.Message
	}
	if err.Cause != nil {
		return err.Cause.Error()
	}
	if err.Type != "" {
		return string(err.Type)
	}
	return "gateway error"
}

func (err *GatewayError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// GatewayType returns the error type.
func (err *GatewayError) GatewayType() GatewayErrorType {
	if err == nil {
		return ""
	}
	return err.Type
}

// GatewayStatusCode returns the HTTP status code.
func (err *GatewayError) GatewayStatusCode() int {
	if err == nil {
		return 0
	}
	return err.StatusCode
}

// GatewayAuthenticationError indicates gateway authentication failures.
type GatewayAuthenticationError struct {
	GatewayError
}

// GatewayInvalidRequestError indicates malformed gateway requests.
type GatewayInvalidRequestError struct {
	GatewayError
}

// GatewayRateLimitError indicates gateway rate limit violations.
type GatewayRateLimitError struct {
	GatewayError
}

// GatewayModelNotFoundError indicates a missing model.
type GatewayModelNotFoundError struct {
	GatewayError
	ModelID string
}

// GatewayInternalServerError indicates gateway server failures.
type GatewayInternalServerError struct {
	GatewayError
}

// GatewayResponseError indicates invalid gateway responses.
type GatewayResponseError struct {
	GatewayError
	Response        any
	ValidationError error
}

// GatewayErrorResponseOptions configures gateway error creation.
type GatewayErrorResponseOptions struct {
	Response       any
	StatusCode     int
	DefaultMessage string
	Cause          error
	AuthMethod     GatewayAuthMethod
}

// GatewayErrorMetadata carries request/response metadata for provider errors.
type GatewayErrorMetadata struct {
	RequestID       string
	ResponseHeaders map[string][]string
	ResponseBody    string
	ProviderID      provider.ProviderID
	ModelID         provider.ModelID
}

// CreateGatewayErrorFromResponse builds a gateway error from a parsed response.
func CreateGatewayErrorFromResponse(options GatewayErrorResponseOptions) error {
	defaultMessage := options.DefaultMessage
	if defaultMessage == "" {
		defaultMessage = "Gateway request failed"
	}
	statusCode := options.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	parsed, err := parseGatewayErrorResponse(options.Response)
	if err != nil {
		return &GatewayResponseError{
			GatewayError: GatewayError{
				Type:       GatewayErrorTypeResponseError,
				Message:    fmt.Sprintf("Invalid error response format: %s", defaultMessage),
				StatusCode: statusCode,
				Cause:      options.Cause,
			},
			Response:        options.Response,
			ValidationError: err,
		}
	}

	message := parsed.Error.Message
	if message == "" {
		message = defaultMessage
	}

	switch GatewayErrorType(parsed.Error.Type) {
	case GatewayErrorTypeAuthentication:
		return newGatewayAuthenticationError(options.AuthMethod, statusCode, options.Cause)
	case GatewayErrorTypeInvalidRequest:
		return &GatewayInvalidRequestError{GatewayError: GatewayError{
			Type:       GatewayErrorTypeInvalidRequest,
			Message:    message,
			StatusCode: statusCode,
			Cause:      options.Cause,
		}}
	case GatewayErrorTypeRateLimit:
		return &GatewayRateLimitError{GatewayError: GatewayError{
			Type:       GatewayErrorTypeRateLimit,
			Message:    message,
			StatusCode: statusCode,
			Cause:      options.Cause,
		}}
	case GatewayErrorTypeModelNotFound:
		return &GatewayModelNotFoundError{GatewayError: GatewayError{
			Type:       GatewayErrorTypeModelNotFound,
			Message:    message,
			StatusCode: statusCode,
			Cause:      options.Cause,
		}, ModelID: extractGatewayModelID(parsed.Error.Param)}
	case GatewayErrorTypeInternalServer:
		return &GatewayInternalServerError{GatewayError: GatewayError{
			Type:       GatewayErrorTypeInternalServer,
			Message:    message,
			StatusCode: statusCode,
			Cause:      options.Cause,
		}}
	default:
		return &GatewayInternalServerError{GatewayError: GatewayError{
			Type:       GatewayErrorTypeInternalServer,
			Message:    message,
			StatusCode: statusCode,
			Cause:      options.Cause,
		}}
	}
}

// AsGatewayError converts provider errors into gateway errors when possible.
func AsGatewayError(err error, authMethod GatewayAuthMethod) error {
	if err == nil {
		return nil
	}

	var gatewayErr interface {
		GatewayType() GatewayErrorType
	}
	if errors.As(err, &gatewayErr) {
		return err
	}

	var apiErr *provider.ApiCallError
	if errors.As(err, &apiErr) {
		return CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
			Response:       ExtractAPICallResponse(apiErr),
			StatusCode:     apiErr.StatusCode,
			DefaultMessage: "Gateway request failed",
			Cause:          err,
			AuthMethod:     authMethod,
		})
	}

	message := "Unknown Gateway error"
	if err != nil {
		message = fmt.Sprintf("Gateway request failed: %s", err.Error())
	}

	return CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
		Response:       map[string]any{},
		StatusCode:     http.StatusInternalServerError,
		DefaultMessage: message,
		Cause:          err,
		AuthMethod:     authMethod,
	})
}

// ExtractAPICallResponse extracts response data from API call errors.
func ExtractAPICallResponse(err *provider.ApiCallError) any {
	if err == nil {
		return map[string]any{}
	}
	if err.ResponseBody == "" {
		return map[string]any{}
	}
	var decoded any
	if json.Unmarshal([]byte(err.ResponseBody), &decoded) == nil {
		return decoded
	}
	return err.ResponseBody
}

// MapGatewayErrorToProvider maps gateway errors to provider error types.
func MapGatewayErrorToProvider(err error, metadata GatewayErrorMetadata) error {
	if err == nil {
		return nil
	}

	var authErr *GatewayAuthenticationError
	if errors.As(err, &authErr) {
		mapped := provider.NewAuthenticationError(authErr.Message, authErr)
		applyApiCallMetadata(&mapped.ApiCallError, metadata, authErr.StatusCode)
		return mapped
	}

	var invalidReqErr *GatewayInvalidRequestError
	if errors.As(err, &invalidReqErr) {
		mapped := provider.NewInvalidRequestError(invalidReqErr.Message, invalidReqErr)
		applyApiCallMetadata(&mapped.ApiCallError, metadata, invalidReqErr.StatusCode)
		return mapped
	}

	var rateLimitErr *GatewayRateLimitError
	if errors.As(err, &rateLimitErr) {
		retryAfter := retryAfterSeconds(metadata.ResponseHeaders)
		mapped := provider.NewRateLimitError(rateLimitErr.Message, rateLimitErr, retryAfter)
		applyApiCallMetadata(&mapped.ApiCallError, metadata, rateLimitErr.StatusCode)
		return mapped
	}

	var modelNotFoundErr *GatewayModelNotFoundError
	if errors.As(err, &modelNotFoundErr) {
		resolvedModel := provider.ModelID(modelNotFoundErr.ModelID)
		if resolvedModel == "" {
			resolvedModel = metadata.ModelID
		}
		mapped := provider.NewNoSuchModelError(modelNotFoundErr.Message, modelNotFoundErr, metadata.ProviderID, resolvedModel)
		attachGatewayMetadata(&mapped.AISDKError, metadata)
		return mapped
	}

	var internalErr *GatewayInternalServerError
	if errors.As(err, &internalErr) {
		mapped := provider.NewInternalServerError(internalErr.Message, internalErr)
		applyApiCallMetadata(&mapped.ApiCallError, metadata, internalErr.StatusCode)
		return mapped
	}

	var responseErr *GatewayResponseError
	if errors.As(err, &responseErr) {
		mapped := provider.NewInvalidResponseDataError(responseErr.Message, responseErr)
		attachGatewayMetadata(&mapped.AISDKError, metadata)
		return mapped
	}

	return err
}

type gatewayErrorResponse struct {
	Error gatewayErrorPayload `json:"error"`
}

type gatewayErrorPayload struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Param   any    `json:"param,omitempty"`
	Code    any    `json:"code,omitempty"`
}

func parseGatewayErrorResponse(response any) (gatewayErrorResponse, error) {
	if response == nil {
		return gatewayErrorResponse{}, errors.New("empty response")
	}

	switch typed := response.(type) {
	case []byte:
		if len(typed) == 0 {
			return gatewayErrorResponse{}, errors.New("empty response")
		}
		return decodeGatewayErrorResponse(typed)
	case string:
		if strings.TrimSpace(typed) == "" {
			return gatewayErrorResponse{}, errors.New("empty response")
		}
		return decodeGatewayErrorResponse([]byte(typed))
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return gatewayErrorResponse{}, err
		}
		return decodeGatewayErrorResponse(payload)
	}
}

func decodeGatewayErrorResponse(payload []byte) (gatewayErrorResponse, error) {
	var parsed gatewayErrorResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return gatewayErrorResponse{}, err
	}
	if parsed.Error.Message == "" {
		return gatewayErrorResponse{}, errors.New("missing error message")
	}
	return parsed, nil
}

func extractGatewayModelID(param any) string {
	switch typed := param.(type) {
	case map[string]any:
		if value, ok := typed["modelId"].(string); ok {
			return value
		}
	case map[string]string:
		if value, ok := typed["modelId"]; ok {
			return value
		}
	case provider.JSONObject:
		if value, ok := typed["modelId"].(string); ok {
			return value
		}
	}
	return ""
}

func newGatewayAuthenticationError(authMethod GatewayAuthMethod, statusCode int, cause error) *GatewayAuthenticationError {
	message := "Authentication failed"
	switch authMethod {
	case GatewayAuthMethodAPIKey:
		message = "AI Gateway authentication failed: Invalid API key.\n\n" +
			"Create a new API key: https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E%2Fai%2Fapi-keys\n\n" +
			"Provide via 'apiKey' option or 'AI_GATEWAY_API_KEY' environment variable."
	case GatewayAuthMethodOIDC:
		message = "AI Gateway authentication failed: Invalid OIDC token.\n\n" +
			"Run 'npx vercel link' to link your project, then 'vc env pull' to fetch the token.\n\n" +
			"Alternatively, use an API key: https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E%2Fai%2Fapi-keys"
	default:
		message = "AI Gateway authentication failed: No authentication provided.\n\n" +
			"Option 1 - API key:\n" +
			"Create an API key: https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E%2Fai%2Fapi-keys\n" +
			"Provide via 'apiKey' option or 'AI_GATEWAY_API_KEY' environment variable.\n\n" +
			"Option 2 - OIDC token:\n" +
			"Run 'npx vercel link' to link your project, then 'vc env pull' to fetch the token."
	}

	return &GatewayAuthenticationError{GatewayError: GatewayError{
		Type:       GatewayErrorTypeAuthentication,
		Message:    message,
		StatusCode: statusCode,
		Cause:      cause,
	}}
}

func applyApiCallMetadata(err *provider.ApiCallError, metadata GatewayErrorMetadata, statusCode int) {
	if err == nil {
		return
	}
	if err.StatusCode == 0 {
		err.StatusCode = statusCode
	}
	if err.RequestID == "" {
		err.RequestID = metadata.RequestID
	}
	if err.ResponseHeaders == nil && metadata.ResponseHeaders != nil {
		err.ResponseHeaders = metadata.ResponseHeaders
	}
	if err.ResponseBody == "" {
		err.ResponseBody = metadata.ResponseBody
	}
	if err.ProviderID == "" {
		err.ProviderID = metadata.ProviderID
	}
	if err.ModelID == "" {
		err.ModelID = metadata.ModelID
	}
}

func attachGatewayMetadata(err *provider.AISDKError, metadata GatewayErrorMetadata) {
	if err == nil {
		return
	}
	if err.Details == nil {
		err.Details = map[string]any{}
	}
	if metadata.RequestID != "" {
		err.Details["request_id"] = metadata.RequestID
	}
	if metadata.ResponseHeaders != nil {
		err.Details["response_headers"] = metadata.ResponseHeaders
	}
	if metadata.ResponseBody != "" {
		err.Details["response_body"] = metadata.ResponseBody
	}
}

func retryAfterSeconds(headers map[string][]string) int {
	if headers == nil {
		return 0
	}
	header := http.Header(headers)
	value := header.Get("Retry-After")
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return seconds
	}
	if parsed, err := http.ParseTime(value); err == nil {
		wait := time.Until(parsed)
		if wait > 0 {
			return int(wait.Seconds())
		}
	}
	return 0
}
