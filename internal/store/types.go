package store

// CreateAccountInput is the input for creating an account.
type CreateAccountInput struct {
	ID           string
	Username     string
	Domain       *string
	DisplayName  *string
	Note         *string
	PublicKey    string
	PrivateKey   *string
	InboxURL     string
	OutboxURL    string
	FollowersURL string
	FollowingURL string
	APID         string
	ApRaw        []byte
	Bot          bool
	Locked       bool
}

// CreateUserInput is the input for creating a user.
type CreateUserInput struct {
	ID           string
	AccountID    string
	Email        string
	PasswordHash string
	Role         string
}

// CreateStatusInput is the input for creating a status.
type CreateStatusInput struct {
	ID             string
	URI            string
	AccountID      string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	ReblogOfID     *string
	APID           string
	ApRaw          []byte
	Sensitive      bool
	Local          bool
}
