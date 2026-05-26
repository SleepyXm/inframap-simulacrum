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
