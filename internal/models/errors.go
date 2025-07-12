package models

import "errors"

var (
	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("resource not found")

	// ErrForbidden is returned when access is denied to a resource.
	ErrForbidden = errors.New("access denied")

	// ErrInactiveAccount is returned when an inactive user tries to log in.
	ErrInactiveAccount = errors.New("user account is not active")

	// ErrInvalidToken is returned when a token is invalid or expired.
	ErrInvalidToken = errors.New("invalid or expired token")

	// ErrConflict is returned when there's a conflict (e.g., duplicate email).
	ErrConflict = errors.New("resource conflict")

	// ErrInvalidCredentials is returned when login credentials are invalid.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrNicknameTaken is returned when a nickname is already in use.
	ErrNicknameTaken = errors.New("nickname is already taken")

	// ErrOrderCannotBeCancelled is returned when an attempt is made to cancel an order
	// that is no longer in a cancellable state (e.g., 'in_transit' or 'delivered').
	ErrOrderCannotBeCancelled = errors.New("order cannot be cancelled")

	// ErrOrderCannotBePaid is returned when an attempt is made to pay for an order
	// that is not in a 'pending' state.
	ErrOrderCannotBePaid = errors.New("order is not in a state that can be paid for")

	// ErrRouteOptionExpired is returned when the user tries to create an order
	// with a route option ID that is expired or invalid.
	ErrRouteOptionExpired = errors.New("the delivery quote has expired, please request a new one")

	// ErrCannotSubmitFeedback is returned when a user tries to submit feedback for an order
	// that is not yet delivered.
	ErrCannotSubmitFeedback = errors.New("feedback can only be submitted for delivered orders")

	// ErrFeedbackAlreadySubmitted is returned when a user tries to submit feedback
	// for an order that already has feedback.
	ErrFeedbackAlreadySubmitted = errors.New("feedback has already been submitted for this order")

	// ErrPackageTooLarge indicates that the weight or dimensions of the requested
	// delivery exceed what our machines can handle.
	ErrPackageTooLarge = errors.New("package exceeds allowed weight or dimensions")
)
