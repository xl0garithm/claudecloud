package provider

import "errors"

var (
	// ErrNotFound indicates the requested instance does not exist.
	ErrNotFound = errors.New("instance not found")

	// ErrAlreadyExists indicates an instance already exists for this user.
	ErrAlreadyExists = errors.New("instance already exists for user")

	// ErrInvalidState indicates the instance is in a state that doesn't allow the requested operation.
	ErrInvalidState = errors.New("invalid instance state for operation")

	// ErrProviderNotConfigured indicates the selected provider is missing required configuration.
	ErrProviderNotConfigured = errors.New("provider not configured")
)
