// Package auth holds the identity model and the contracts for issuing and
// verifying access tokens.
package auth

// User is an account that owns links. Its password hash is never serialised
// onto a transport.
type User struct{}

// Credentials is a login attempt.
type Credentials struct{}

// Claims is the verified content of an access token.
type Claims struct{}

// Token is an issued access token and its expiry.
type Token struct{}
