package aichat

import "errors"

var (
	// ErrNotFound indicates a resource was not found.
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput indicates invalid input was provided.
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized indicates the request was unauthorized.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrConversationNotFound indicates the conversation was not found.
	ErrConversationNotFound = errors.New("conversation not found")

	// ErrNoEntityIdentifier indicates no entity identifier was provided.
	ErrNoEntityIdentifier = errors.New("no entity identifier provided")

	// ErrExpertNotFound indicates the requested expert was not found.
	ErrExpertNotFound = errors.New("expert not found")
)
