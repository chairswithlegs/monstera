package apimodel

// AdminPendingRegistration is one pending registration in the admin API.
type AdminPendingRegistration struct {
	UserID             string  `json:"user_id"`
	AccountID          string  `json:"account_id"`
	Email              string  `json:"email"`
	Username           string  `json:"username"`
	RegistrationReason *string `json:"registration_reason,omitempty"`
}

// AdminPendingRegistrationList is the response for GET /admin/registrations.
type AdminPendingRegistrationList struct {
	Pending []AdminPendingRegistration `json:"pending"`
}
