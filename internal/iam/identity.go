package iam

// Identity represents an authenticated caller.
type Identity struct {
	AccessKey string
	UserID    string
	IsAdmin   bool
	Policies  []Policy
}
