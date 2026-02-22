// Package student contains all HTTP handlers related to the Student resource.
//
// HANDLER PATTERN USED HERE — THE CLOSURE / FACTORY PATTERN:
// ────────────────────────────────────────────────────────────
// Go's router expects handler functions with the signature:
//
//	func(http.ResponseWriter, *http.Request)
//
// That signature has no room for extra parameters like a database.
// To inject dependencies we use a factory function that:
//  1. Accepts dependencies (storage)
//  2. Returns a function with the exact signature the router needs
//
// Because the inner function "closes over" the outer parameters, it can
// access `storage` even after the factory call has returned.
// This is called a closure. Example:
//
//	router.HandleFunc("POST /api/students", student.New(storage))
//	//                                              ^^^^^^^^^^^^^
//	//                         New(storage) is called ONCE at startup.
//	//                         It returns a handler func which is called
//	//                         on EVERY incoming request.
package student

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/aanand-mishra/students-api/internal/storage"
	"github.com/aanand-mishra/students-api/internal/types"
	"github.com/aanand-mishra/students-api/internal/utils/response"
	"github.com/go-playground/validator/v10"
)

// ─────────────────────────────────────────────────────────────────────────────
// New handles POST /api/students
// Creates a new student from the JSON request body.
//
// Request body (JSON):
//
//	{ "name": "Rakesh", "email": "rakesh@test.com", "age": 35 }
//
// Success response (201 Created):
//
//	{ "id": 1 }
//
// Error responses:
//
//	400 Bad Request  — empty body, malformed JSON, or failed validation
//	500 Internal     — database error
//
// ─────────────────────────────────────────────────────────────────────────────
func New(storage storage.Storage) http.HandlerFunc {
	// This is the factory function. It runs ONCE when the route is registered.
	// It captures `storage` in the closure below.

	return func(w http.ResponseWriter, r *http.Request) {
		// Structured log: every request gets an Info log so we can trace
		// activity in production logs.
		slog.Info("creating a student")

		// ── Step 1: Decode JSON body into a Student struct ────────────
		var student types.Student

		// json.NewDecoder reads from r.Body (the raw bytes sent by the client).
		// .Decode(&student) populates the student variable via its pointer.
		// Fields in the JSON are matched to struct fields using json:"..." tags.
		err := json.NewDecoder(r.Body).Decode(&student)

		if errors.Is(err, io.EOF) {
			// io.EOF means the body was completely empty — nothing to decode.
			response.WriteJSON(w, http.StatusBadRequest,
				response.GeneralError(errors.New("request body is empty")))
			return // stop further processing
		}

		if err != nil {
			// Any other decode error: malformed JSON, wrong types, etc.
			response.WriteJSON(w, http.StatusBadRequest, response.GeneralError(err))
			return
		}

		// ── Step 2: Validate the decoded struct ───────────────────────
		// validator.New().Struct(v) checks all validate:"..." tags on v.
		// It returns nil if everything is valid, or a ValidationErrors
		// (which implements the error interface) if any rule fails.
		if err := validator.New().Struct(student); err != nil {
			// Type-assert the error to ValidationErrors so we can inspect
			// each individual field error (field name, broken tag, etc.).
			validateErrs := err.(validator.ValidationErrors)
			response.WriteJSON(w, http.StatusBadRequest,
				response.ValidationError(validateErrs))
			return
		}

		// ── Step 3: Persist to database ───────────────────────────────
		// We call the Storage interface method — not SQLite directly.
		// This keeps the handler database-agnostic.
		lastID, err := storage.CreateStudent(student.Name, student.Email, student.Age)
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.GeneralError(err))
			return
		}

		slog.Info("student created", slog.Int64("id", lastID))

		// ── Step 4: Return 201 Created with the new student's ID ──────
		// map[string]int64 encodes to: {"id": 1}
		response.WriteJSON(w, http.StatusCreated, map[string]int64{"id": lastID})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetByID handles GET /api/students/{id}
// Fetches a single student by their primary key ID.
//
// Path parameter: {id} — must be a valid integer
//
// Success response (200 OK):
//
//	{ "id": 1, "name": "Rakesh", "email": "rakesh@test.com", "age": 35 }
//
// Error responses:
//
//	400 Bad Request  — id is not a valid integer
//	500 Internal     — database error or student not found
//
// ─────────────────────────────────────────────────────────────────────────────
func GetByID(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// r.PathValue("id") extracts the {id} segment from the URL.
		// This works because Go 1.22+ supports named path parameters in
		// the ServeMux pattern: "GET /api/students/{id}"
		id := r.PathValue("id")
		slog.Info("getting a student", slog.String("id", id))

		// The URL gives us a string; the database needs int64.
		// strconv.ParseInt(s, base, bitSize) converts string → int64.
		// base 10 = decimal, bitSize 64 = int64.
		intID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			// The client sent something like "/api/students/abc"
			response.WriteJSON(w, http.StatusBadRequest,
				response.GeneralError(errors.New("invalid id: must be an integer")))
			return
		}

		student, err := storage.GetStudentByID(intID)
		if err != nil {
			slog.Error("error getting student",
				slog.String("id", id),
				slog.String("error", err.Error()))
			response.WriteJSON(w, http.StatusInternalServerError,
				response.GeneralError(err))
			return
		}

		response.WriteJSON(w, http.StatusOK, student)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetList handles GET /api/students
// Returns a JSON array of all students in the database.
//
// Success response (200 OK):
//
//	[
//	  { "id": 1, "name": "Rakesh", ... },
//	  { "id": 2, "name": "Priya",  ... }
//	]
//
// Returns an empty array [] (not null) when there are no students.
// ─────────────────────────────────────────────────────────────────────────────
func GetList(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("getting all students")

		students, err := storage.GetStudents()
		if err != nil {
			slog.Error("error getting students", slog.String("error", err.Error()))
			response.WriteJSON(w, http.StatusInternalServerError,
				response.GeneralError(err))
			return
		}

		response.WriteJSON(w, http.StatusOK, students)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Update handles PUT /api/students/{id}
// Replaces ALL fields of an existing student.
//
// Request body (JSON) — all fields required for a PUT:
//
//	{ "name": "Rakesh Updated", "email": "new@test.com", "age": 36 }
//
// Success response (200 OK) — the updated student:
//
//	{ "id": 1, "name": "Rakesh Updated", "email": "new@test.com", "age": 36 }
//
// Error responses:
//
//	400 Bad Request  — invalid id, empty body, or validation failure
//	500 Internal     — database error
//
// ─────────────────────────────────────────────────────────────────────────────
func Update(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		slog.Info("updating a student", slog.String("id", id))

		intID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			response.WriteJSON(w, http.StatusBadRequest,
				response.GeneralError(errors.New("invalid id: must be an integer")))
			return
		}

		// Decode the update payload
		var student types.Student
		err = json.NewDecoder(r.Body).Decode(&student)
		if errors.Is(err, io.EOF) {
			response.WriteJSON(w, http.StatusBadRequest,
				response.GeneralError(errors.New("request body is empty")))
			return
		}
		if err != nil {
			response.WriteJSON(w, http.StatusBadRequest, response.GeneralError(err))
			return
		}

		// Validate the update payload using the same rules as creation
		if err := validator.New().Struct(student); err != nil {
			validateErrs := err.(validator.ValidationErrors)
			response.WriteJSON(w, http.StatusBadRequest,
				response.ValidationError(validateErrs))
			return
		}

		// Persist and retrieve the updated record
		updated, err := storage.UpdateStudentByID(intID, student)
		if err != nil {
			slog.Error("error updating student",
				slog.String("id", id),
				slog.String("error", err.Error()))
			response.WriteJSON(w, http.StatusInternalServerError,
				response.GeneralError(err))
			return
		}

		slog.Info("student updated", slog.String("id", id))
		response.WriteJSON(w, http.StatusOK, updated)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete handles DELETE /api/students/{id}
// Permanently removes a student record from the database.
//
// Success response (200 OK):
//
//	{ "status": "deleted" }
//
// Error responses:
//
//	400 Bad Request  — invalid id
//	500 Internal     — database error
//
// ─────────────────────────────────────────────────────────────────────────────
func Delete(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		slog.Info("deleting a student", slog.String("id", id))

		intID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			response.WriteJSON(w, http.StatusBadRequest,
				response.GeneralError(errors.New("invalid id: must be an integer")))
			return
		}

		if err := storage.DeleteStudentByID(intID); err != nil {
			slog.Error("error deleting student",
				slog.String("id", id),
				slog.String("error", err.Error()))
			response.WriteJSON(w, http.StatusInternalServerError,
				response.GeneralError(err))
			return
		}

		slog.Info("student deleted", slog.String("id", id))
		response.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
