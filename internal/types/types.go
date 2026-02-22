// Package types holds all shared data structures (models) used across
// the application. Keeping them in one place prevents import cycles —
// handlers, storage, and utils can all import types without depending
// on each other.
package types

// Student represents a student record in our system.
//
// Struct tags serve two purposes:
//
//  1. json:"..."  — controls how the field appears when encoded to JSON
//     (lowercase names match REST API conventions).
//     Without this tag Go uses the exported field name, e.g. "Name".
//
//  2. validate:"..." — rules checked by the go-playground/validator
//     package. "required" means the field must be non-zero / non-empty.
type Student struct {
	ID    int    `json:"id"`
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required"`
	Age   int    `json:"age"   validate:"required"`
}
