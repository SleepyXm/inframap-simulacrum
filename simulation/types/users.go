package types

type VirtualUser struct {
	ID    string
	Seed  map[string]string // keyed credential/profile data from sim start
	State VUState
}

type VUState struct {
	Authenticated bool
	LastEndpoint  string
	LastStatus    int
	Session       map[string]string // tokens, cookies etc
}

type TokenSource string

type AuthConfig struct {
	// where to find the token after login
	TokenSource TokenSource
	TokenKey    string // "token", "access_token", "jwt", whatever the key is
	HeaderName  string // if writing to a header on subsequent requests e.g. "Authorization"
	Prefix      string // "Bearer ", "Token ", or empty
}

const (
	TokenFromBody   TokenSource = "body"   // JSON response body
	TokenFromHeader TokenSource = "header" // response header e.g. Set-Cookie
)
