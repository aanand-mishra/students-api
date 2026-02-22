// Package storage defines the Storage interface — a contract that any
// database backend must satisfy to work with this application.
//
// WHY AN INTERFACE?
// ─────────────────
// Handlers (HTTP layer) should not know or care which database they are
// talking to. By depending only on this interface:
//
//   - Switching databases = implement the interface for the new DB,
//     change one line in main.go. Zero handler changes.
//
//   - Writing tests = pass a fake/mock that satisfies the interface.
//     No real database needed for unit tests.
//
// This is the Dependency Inversion Principle in practice.
package storage

import "github.com/aanand-mishra/students-api/internal/types"

// Storage is the database contract.
// Any concrete type that implements ALL of these methods automatically
// satisfies this interface — Go does this implicitly (no "implements"
// keyword required).
type Storage interface {
	// CreateStudent inserts a new student record and returns the auto-
	// generated primary-key ID. Returns an error on failure.
	CreateStudent(name string, email string, age int) (int64, error)

	// GetStudentByID fetches a single student by their primary key.
	// Returns an error (with a descriptive message) if not found.
	GetStudentByID(id int64) (types.Student, error)

	// GetStudents returns every student in the database.
	// Returns an empty slice (not nil) if there are no students.
	GetStudents() ([]types.Student, error)

	// UpdateStudentByID replaces the fields of an existing student.
	// Returns the updated student record or an error.
	UpdateStudentByID(id int64, student types.Student) (types.Student, error)

	// DeleteStudentByID removes a student record permanently.
	DeleteStudentByID(id int64) error
}
