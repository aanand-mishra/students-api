// Package sqlite provides a SQLite-backed implementation of the
// storage.Storage interface using Go's standard database/sql package.
//
// WHY SQLite?
// ───────────
// SQLite stores everything in a single file on disk. There is no
// network, no separate server process, and no installation beyond the
// driver. It is fast enough for most projects and trivial to set up.
//
// The blank import below registers the sqlite3 driver with database/sql.
// The driver's init() function does this automatically when the package
// is loaded — we never call anything from it directly.
package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/aanand-mishra/students-api/internal/config"
	"github.com/aanand-mishra/students-api/internal/types"

	// Blank import: side-effect only (registers the "sqlite3" driver).
	// Without this the sql.Open("sqlite3", ...) call would fail with
	// "unknown driver".
	_ "github.com/mattn/go-sqlite3"
)

// SQLite is the concrete implementation of storage.Storage.
// It holds a *sql.DB which is a connection pool managed by database/sql.
// A single *sql.DB is safe for concurrent use by multiple goroutines.
type SQLite struct {
	Db *sql.DB
}

// New opens the SQLite database at the path specified in cfg.StoragePath,
// creates the students table if it does not already exist, and returns
// a ready-to-use *SQLite.
//
// Naming convention: New() acts as a constructor. Go has no constructors,
// so the community convention is a package-level New() function that
// returns an initialised instance (and an error as the second value).
func New(cfg *config.Config) (*SQLite, error) {
	// sql.Open does NOT open a real connection yet — it just validates
	// the driver name and data source name (DSN).
	// The first actual connection happens on the first query.
	db, err := sql.Open("sqlite3", cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("sqlite.New: open db: %w", err)
	}

	// CREATE TABLE IF NOT EXISTS is idempotent — safe to run on every
	// startup. If the table already exists nothing happens.
	//
	// Schema:
	//   id    — integer primary key, auto-incremented by SQLite
	//   name  — student's full name (TEXT = variable-length string)
	//   email — student's email address
	//   age   — student's age in years
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS students (
			id    INTEGER PRIMARY KEY AUTOINCREMENT,
			name  TEXT    NOT NULL,
			email TEXT    NOT NULL,
			age   INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite.New: create table: %w", err)
	}

	return &SQLite{Db: db}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CreateStudent inserts a new row into the students table.
//
// HOW PREPARED STATEMENTS PREVENT SQL INJECTION:
// ────────────────────────────────────────────────
// If we built the query by concatenating user input:
//
//	query := "INSERT ... VALUES ('" + name + "')"
//
// A malicious user could send: name = "'; DROP TABLE students; --"
// and destroy the database.
//
// Prepared statements use placeholders (?). The database driver sends
// the query and the values separately. The database engine treats the
// values as pure data, never as SQL syntax.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SQLite) CreateStudent(name, email string, age int) (int64, error) {
	// Prepare compiles the SQL on the database side.
	// The ? placeholders will be filled in when we call Exec.
	stmt, err := s.Db.Prepare(
		"INSERT INTO students (name, email, age) VALUES (?, ?, ?)",
	)
	if err != nil {
		return 0, fmt.Errorf("CreateStudent: prepare: %w", err)
	}
	// defer ensures the statement is closed when this function returns,
	// even if we return early due to an error. Prevents resource leaks.
	defer stmt.Close()

	// Exec runs the prepared statement, substituting ? in the same order
	// the arguments are listed here. Order matters!
	result, err := stmt.Exec(name, email, age)
	if err != nil {
		return 0, fmt.Errorf("CreateStudent: exec: %w", err)
	}

	// LastInsertId returns the auto-generated primary key of the new row.
	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("CreateStudent: last insert id: %w", err)
	}

	return lastID, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetStudentByID fetches exactly one student row matched by primary key.
//
// HOW QueryRow + Scan WORK:
// ──────────────────────────
// QueryRow executes the query and returns a *Row — a single-row result.
// Scan reads the columns from that row into Go variables IN ORDER.
// The order of variables in Scan must match the order of columns in SELECT.
// We pass pointers (&student.ID) so Scan can write into those locations.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SQLite) GetStudentByID(id int64) (types.Student, error) {
	stmt, err := s.Db.Prepare(
		"SELECT id, name, email, age FROM students WHERE id = ? LIMIT 1",
	)
	if err != nil {
		return types.Student{}, fmt.Errorf("GetStudentByID: prepare: %w", err)
	}
	defer stmt.Close()

	var student types.Student

	// QueryRow returns exactly one row. If the query finds no match it
	// does NOT return nil — the error surfaces only when you call Scan.
	err = stmt.QueryRow(id).Scan(
		&student.ID,    // ← maps to SELECT column 1: id
		&student.Name,  // ← maps to SELECT column 2: name
		&student.Email, // ← maps to SELECT column 3: email
		&student.Age,   // ← maps to SELECT column 4: age
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// sql.ErrNoRows is the sentinel error for "nothing matched".
			// We return a human-readable message so the handler can surface
			// it to the client without leaking internal DB details.
			return types.Student{}, fmt.Errorf("no student found with id: %d", id)
		}
		return types.Student{}, fmt.Errorf("GetStudentByID: scan: %w", err)
	}

	return student, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetStudents returns all student rows as a slice.
//
// HOW Query + rows.Next() WORK:
// ──────────────────────────────
// Query (unlike QueryRow) returns *sql.Rows — a cursor over multiple rows.
// We iterate with rows.Next() which advances the cursor and returns false
// when there are no more rows. We Scan each row inside the loop.
// Always defer rows.Close() to release the database connection.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SQLite) GetStudents() ([]types.Student, error) {
	stmt, err := s.Db.Prepare(
		// Explicitly list columns — never use SELECT * in production code.
		// If a column is added later, SELECT * would break Scan's ordering.
		"SELECT id, name, email, age FROM students",
	)
	if err != nil {
		return nil, fmt.Errorf("GetStudents: prepare: %w", err)
	}
	defer stmt.Close()

	// Query returns a cursor (*sql.Rows) over the result set.
	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("GetStudents: query: %w", err)
	}
	defer rows.Close() // must close rows to free the DB connection

	// Pre-allocate an empty (non-nil) slice.
	// Returning [] instead of null in JSON is better API behaviour.
	students := make([]types.Student, 0)

	for rows.Next() { // advances cursor; returns false when exhausted
		var student types.Student

		if err := rows.Scan(
			&student.ID,
			&student.Name,
			&student.Email,
			&student.Age,
		); err != nil {
			return nil, fmt.Errorf("GetStudents: scan row: %w", err)
		}

		students = append(students, student)
	}

	// rows.Err() captures any error that occurred during iteration.
	// This is separate from Scan errors.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetStudents: rows iteration: %w", err)
	}

	return students, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateStudentByID replaces a student's data with the provided values.
// Returns the updated student so the caller can echo it back to the client.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SQLite) UpdateStudentByID(id int64, student types.Student) (types.Student, error) {
	stmt, err := s.Db.Prepare(
		"UPDATE students SET name = ?, email = ?, age = ? WHERE id = ?",
	)
	if err != nil {
		return types.Student{}, fmt.Errorf("UpdateStudentByID: prepare: %w", err)
	}
	defer stmt.Close()

	// Note the argument order matches the ? order in the SQL:
	//   name, email, age, id
	_, err = stmt.Exec(student.Name, student.Email, student.Age, id)
	if err != nil {
		return types.Student{}, fmt.Errorf("UpdateStudentByID: exec: %w", err)
	}

	// Re-fetch the record so we return exactly what is stored in the DB.
	return s.GetStudentByID(id)
}

// ─────────────────────────────────────────────────────────────────────────────
// DeleteStudentByID removes a student row by primary key.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SQLite) DeleteStudentByID(id int64) error {
	stmt, err := s.Db.Prepare("DELETE FROM students WHERE id = ?")
	if err != nil {
		return fmt.Errorf("DeleteStudentByID: prepare: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(id)
	if err != nil {
		return fmt.Errorf("DeleteStudentByID: exec: %w", err)
	}

	return nil
}
