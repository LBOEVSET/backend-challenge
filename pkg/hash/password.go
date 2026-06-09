// Package hash provides bcrypt password hashing utilities.
package hash

import "golang.org/x/crypto/bcrypt"

const cost = bcrypt.DefaultCost

// Password hashes a plaintext password with bcrypt.
func Password(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	return string(b), err
}

// CheckPassword returns true when plain matches the bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
