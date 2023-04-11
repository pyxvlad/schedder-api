package schedder

// CtxKey is used as the key for context values.
type CtxKey int

const (
	// CtxJSON is used for storing the parsed JSON in a context.
	CtxJSON = CtxKey(1)
	// CtxAuthenticatedID represents the authenticated account's ID.
	CtxAuthenticatedID = CtxKey(2)
	// CtxSessionID is used when an endpoint needs a sessionID URL parameter.
	CtxSessionID = CtxKey(3)
	// CtxAccountID is used when an endpoint needs a accountID URL parameter.
	CtxAccountID = CtxKey(4)
	// CtxTenantID is used when an endpoint needs a tenantID URL parameter.
	CtxTenantID = CtxKey(5)

	// BcryptRounds represents the number of rounds to be used in bcrypt.
	BcryptRounds = 10
	// PhoneLength represents the number of characters in an international
	// phone number, like "+40 743 123 123".
	PhoneLength = 12

	// minimumLengthForDevice is the minimum length of the device name.
	minimumLengthForDevice = 8
)
