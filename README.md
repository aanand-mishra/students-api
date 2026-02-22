# Students API

A REST API built with Go that can create, read, update and delete student records. I built this while learning Go and backend development.

It uses SQLite so there's no database server to set up — just run it and it works.

---

## What it does

- Add a new student
- Get a student by their ID
- Get a list of all students
- Update a student's information
- Delete a student

---

## Tech used

- **Go 1.22+**
- **SQLite** (via go-sqlite3) — database stored in a single file
- **cleanenv** — reads config from a YAML file
- **go-playground/validator** — validates incoming request data

---

## Project structure

```
students-api/
├── cmd/students-api/main.go          # entry point, starts the server
├── config/local.yaml                 # config file (port, db path etc.)
├── internal/
│   ├── config/config.go              # loads the yaml config
│   ├── types/types.go                # Student struct
│   ├── storage/storage.go            # storage interface
│   ├── storage/sqlite/sqlite.go      # sqlite implementation
│   ├── http/handlers/student/        # all the route handlers
│   └── utils/response/response.go   # json response helpers
├── go.mod
└── Makefile
```

---

## Getting started

### Requirements

- Go 1.22 or higher → https://go.dev/dl/
- A C compiler (needed for the SQLite driver)
  - macOS: `xcode-select --install`
  - Ubuntu: `sudo apt-get install gcc`

### 1. Clone the repo

```bash
git clone https://github.com/yourusername/students-api.git
cd students-api
```

### 2. Download dependencies

```bash
go mod download
```

### 3. Create the storage folder

```bash
mkdir -p storage
```

### 4. Run the server

```bash
go run ./cmd/students-api --config=config/local.yaml
```

You should see:

```
level=INFO msg="starting students-api" env=dev
level=INFO msg="storage initialised" path=storage/storage.db
level=INFO msg="server started" address=localhost:8082
```

Server is now running at `http://localhost:8082`

---

## API Endpoints

| Method | URL | What it does |
|--------|-----|--------------|
| POST | `/api/students` | Create a student |
| GET | `/api/students` | Get all students |
| GET | `/api/students/{id}` | Get one student |
| PUT | `/api/students/{id}` | Update a student |
| DELETE | `/api/students/{id}` | Delete a student |

---

## Example requests

**Create a student**
```bash
curl -X POST http://localhost:8082/api/students \
  -H "Content-Type: application/json" \
  -d '{"name":"Rakesh","email":"rakesh@test.com","age":35}'
```
```json
{"id": 1}
```

**Get all students**
```bash
curl http://localhost:8082/api/students
```
```json
[
  {"id": 1, "name": "Rakesh", "email": "rakesh@test.com", "age": 35}
]
```

**Get one student**
```bash
curl http://localhost:8082/api/students/1
```
```json
{"id": 1, "name": "Rakesh", "email": "rakesh@test.com", "age": 35}
```

**Update a student**
```bash
curl -X PUT http://localhost:8082/api/students/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Rakesh Kumar","email":"new@test.com","age":36}'
```
```json
{"id": 1, "name": "Rakesh Kumar", "email": "new@test.com", "age": 36}
```

**Delete a student**
```bash
curl -X DELETE http://localhost:8082/api/students/1
```
```json
{"status": "deleted"}
```

---

## Config

The config lives in `config/local.yaml`:

```yaml
env: "dev"
storage_path: "storage/storage.db"
http_server:
  address: "localhost:8082"
```

You can also pass the config path as an environment variable instead of a flag:

```bash
CONFIG_PATH=config/local.yaml go run ./cmd/students-api
```

---

## Build a binary

```bash
CGO_ENABLED=1 go build -o out/students-api ./cmd/students-api
./out/students-api --config=config/local.yaml
```

Or with make:

```bash
make build
make run-binary
```

---

## Things I learned building this

- How REST APIs and CRUD operations work
- Structuring a Go project properly (cmd/, internal/)
- Using interfaces to keep database code separate from handler code
- Graceful shutdown so requests don't get cut off
- Prepared SQL statements to prevent SQL injection
- Reading config from YAML files instead of hardcoding values

---

## License

MIT