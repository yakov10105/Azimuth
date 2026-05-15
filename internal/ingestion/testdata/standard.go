package main

import (
	"fmt"
	"strings"
)

// User represents an application user.
type User struct {
	Name  string
	Email string
	Age   int
}

// Greeter defines greeting behaviour.
type Greeter interface {
	Greet(name string) string
	Farewell(name string) error
}

// NewUser constructs a User from name and email.
func NewUser(name, email string) *User {
	return &User{Name: name, Email: email}
}

// greet returns an informal greeting for a user.
func greet(u *User) string {
	return "Hello, " + u.Name
}

// String returns a human-readable representation.
func (u *User) String() string {
	return fmt.Sprintf("%s <%s>", u.Name, u.Email)
}

// Validate checks that the user fields are non-empty.
func (u *User) Validate() error {
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
