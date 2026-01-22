package provider

import "errors"

// ErrorCategory identifies the high-level category of a provider error.
type ErrorCategory string

const (
	ErrorCategoryAPICall ErrorCategory = "api_call"
	ErrorCategorySDK     ErrorCategory = "sdk"
)

// ErrorKind identifies the specific class of an error.
type ErrorKind string

const (
	ErrorKindAPICall                  ErrorKind = "api_call"
	ErrorKindAuthentication           ErrorKind = "authentication"
	ErrorKindRateLimit                ErrorKind = "rate_limit"
	ErrorKindInvalidRequest           ErrorKind = "invalid_request"
	ErrorKindInternalServer           ErrorKind = "internal_server"
	ErrorKindInvalidPrompt            ErrorKind = "invalid_prompt"
	ErrorKindInvalidResponseData      ErrorKind = "invalid_response_data"
	ErrorKindNoSuchModel              ErrorKind = "no_such_model"
	ErrorKindUnsupportedFunctionality ErrorKind = "unsupported_functionality"
)

// AISDKError is the base error type for the AI SDK.
type AISDKError struct {
	Category ErrorCategory
	Kind     ErrorKind
	Message  string
	Cause    error
	Details  map[string]any
}

func (e *AISDKError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	if e.Kind != "" {
		return string(e.Kind)
	}
	if e.Category != "" {
		return string(e.Category)
	}
	return "ai-sdk error"
}

func (e *AISDKError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *AISDKError) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	match, ok := target.(*AISDKError)
	if !ok {
		return false
	}
	if match.Kind != "" {
		return e.Kind == match.Kind
	}
	if match.Category != "" {
		return e.Category == match.Category
	}
	return false
}

// ApiCallError captures provider API call failures.
type ApiCallError struct {
	AISDKError
	StatusCode      int
	RequestID       string
	ResponseHeaders map[string][]string
	ResponseBody    string
	ProviderID      ProviderID
	ModelID         ModelID
}

// AuthenticationError indicates provider auth failures.
type AuthenticationError struct {
	ApiCallError
}

// RateLimitError indicates provider rate limits.
type RateLimitError struct {
	ApiCallError
	RetryAfterSeconds int
}

// InvalidRequestError indicates a malformed provider request.
type InvalidRequestError struct {
	ApiCallError
}

// InternalServerError indicates provider-side failures.
type InternalServerError struct {
	ApiCallError
}

// InvalidPromptError indicates invalid prompt content.
type InvalidPromptError struct {
	AISDKError
}

// InvalidResponseDataError indicates unexpected provider response data.
type InvalidResponseDataError struct {
	AISDKError
}

// NoSuchModelError indicates a requested model is unavailable.
type NoSuchModelError struct {
	AISDKError
	ProviderID ProviderID
	ModelID    ModelID
}

// UnsupportedFunctionalityError indicates a missing capability.
type UnsupportedFunctionalityError struct {
	AISDKError
	Feature string
}

// Sentinel errors for classification with errors.Is.
var (
	ErrAPICall                  = &AISDKError{Category: ErrorCategoryAPICall}
	ErrSDK                      = &AISDKError{Category: ErrorCategorySDK}
	ErrAuthentication           = &AISDKError{Kind: ErrorKindAuthentication}
	ErrRateLimit                = &AISDKError{Kind: ErrorKindRateLimit}
	ErrInvalidRequest           = &AISDKError{Kind: ErrorKindInvalidRequest}
	ErrInternalServer           = &AISDKError{Kind: ErrorKindInternalServer}
	ErrInvalidPrompt            = &AISDKError{Kind: ErrorKindInvalidPrompt}
	ErrInvalidResponseData      = &AISDKError{Kind: ErrorKindInvalidResponseData}
	ErrNoSuchModel              = &AISDKError{Kind: ErrorKindNoSuchModel}
	ErrUnsupportedFunctionality = &AISDKError{Kind: ErrorKindUnsupportedFunctionality}
)

// NewApiCallError builds an ApiCallError.
func NewApiCallError(message string, cause error) *ApiCallError {
	return &ApiCallError{
		AISDKError: AISDKError{
			Category: ErrorCategoryAPICall,
			Kind:     ErrorKindAPICall,
			Message:  message,
			Cause:    cause,
		},
	}
}

// NewAuthenticationError builds an AuthenticationError.
func NewAuthenticationError(message string, cause error) *AuthenticationError {
	return &AuthenticationError{
		ApiCallError: ApiCallError{
			AISDKError: AISDKError{
				Category: ErrorCategoryAPICall,
				Kind:     ErrorKindAuthentication,
				Message:  message,
				Cause:    cause,
			},
		},
	}
}

// NewRateLimitError builds a RateLimitError.
func NewRateLimitError(message string, cause error, retryAfterSeconds int) *RateLimitError {
	return &RateLimitError{
		ApiCallError: ApiCallError{
			AISDKError: AISDKError{
				Category: ErrorCategoryAPICall,
				Kind:     ErrorKindRateLimit,
				Message:  message,
				Cause:    cause,
			},
		},
		RetryAfterSeconds: retryAfterSeconds,
	}
}

// NewInvalidRequestError builds an InvalidRequestError.
func NewInvalidRequestError(message string, cause error) *InvalidRequestError {
	return &InvalidRequestError{
		ApiCallError: ApiCallError{
			AISDKError: AISDKError{
				Category: ErrorCategoryAPICall,
				Kind:     ErrorKindInvalidRequest,
				Message:  message,
				Cause:    cause,
			},
		},
	}
}

// NewInternalServerError builds an InternalServerError.
func NewInternalServerError(message string, cause error) *InternalServerError {
	return &InternalServerError{
		ApiCallError: ApiCallError{
			AISDKError: AISDKError{
				Category: ErrorCategoryAPICall,
				Kind:     ErrorKindInternalServer,
				Message:  message,
				Cause:    cause,
			},
		},
	}
}

// NewInvalidPromptError builds an InvalidPromptError.
func NewInvalidPromptError(message string, cause error) *InvalidPromptError {
	return &InvalidPromptError{
		AISDKError: AISDKError{
			Category: ErrorCategorySDK,
			Kind:     ErrorKindInvalidPrompt,
			Message:  message,
			Cause:    cause,
		},
	}
}

// NewInvalidResponseDataError builds an InvalidResponseDataError.
func NewInvalidResponseDataError(message string, cause error) *InvalidResponseDataError {
	return &InvalidResponseDataError{
		AISDKError: AISDKError{
			Category: ErrorCategorySDK,
			Kind:     ErrorKindInvalidResponseData,
			Message:  message,
			Cause:    cause,
		},
	}
}

// NewNoSuchModelError builds a NoSuchModelError.
func NewNoSuchModelError(message string, cause error, providerID ProviderID, modelID ModelID) *NoSuchModelError {
	return &NoSuchModelError{
		AISDKError: AISDKError{
			Category: ErrorCategorySDK,
			Kind:     ErrorKindNoSuchModel,
			Message:  message,
			Cause:    cause,
		},
		ProviderID: providerID,
		ModelID:    modelID,
	}
}

// NewUnsupportedFunctionalityError builds an UnsupportedFunctionalityError.
func NewUnsupportedFunctionalityError(message string, cause error, feature string) *UnsupportedFunctionalityError {
	return &UnsupportedFunctionalityError{
		AISDKError: AISDKError{
			Category: ErrorCategorySDK,
			Kind:     ErrorKindUnsupportedFunctionality,
			Message:  message,
			Cause:    cause,
		},
		Feature: feature,
	}
}

// IsAPICallError reports whether an error is in the API call category.
func IsAPICallError(err error) bool {
	return errors.Is(err, ErrAPICall)
}

// IsSDKError reports whether an error is in the SDK category.
func IsSDKError(err error) bool {
	return errors.Is(err, ErrSDK)
}

// IsAuthenticationError reports whether an error is an AuthenticationError.
func IsAuthenticationError(err error) bool {
	return errors.Is(err, ErrAuthentication)
}

// IsRateLimitError reports whether an error is a RateLimitError.
func IsRateLimitError(err error) bool {
	return errors.Is(err, ErrRateLimit)
}

// IsInvalidRequestError reports whether an error is an InvalidRequestError.
func IsInvalidRequestError(err error) bool {
	return errors.Is(err, ErrInvalidRequest)
}

// IsInternalServerError reports whether an error is an InternalServerError.
func IsInternalServerError(err error) bool {
	return errors.Is(err, ErrInternalServer)
}

// IsInvalidPromptError reports whether an error is an InvalidPromptError.
func IsInvalidPromptError(err error) bool {
	return errors.Is(err, ErrInvalidPrompt)
}

// IsInvalidResponseDataError reports whether an error is an InvalidResponseDataError.
func IsInvalidResponseDataError(err error) bool {
	return errors.Is(err, ErrInvalidResponseData)
}

// IsNoSuchModelError reports whether an error is a NoSuchModelError.
func IsNoSuchModelError(err error) bool {
	return errors.Is(err, ErrNoSuchModel)
}

// IsUnsupportedFunctionalityError reports whether an error is an UnsupportedFunctionalityError.
func IsUnsupportedFunctionalityError(err error) bool {
	return errors.Is(err, ErrUnsupportedFunctionality)
}
