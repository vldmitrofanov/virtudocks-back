package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db             *sql.DB
	exportPassword string
)

type submissionInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func main() {
	var err error

	// Load password from environment
	exportPassword = os.Getenv("EXPORT_PASSWORD")
	if exportPassword == "" {
		log.Println("[WARN] EXPORT_PASSWORD is not set — /export will be UNPROTECTED!")
	}

	// Open (or create) SQLite DB
	db, err = sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// Create table if not exists
	createTable := `
CREATE TABLE IF NOT EXISTS submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT NOT NULL,
    exported INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatalf("create table: %v", err)
	}

	// Simple health/root endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("OK\n"))
	})

	http.HandleFunc("/submit", handleSubmit)
	http.HandleFunc("/export", handleExport)

	addr := ":8080"
	log.Println("Starting server on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("ListenAndServe error: %v", err)
	}
}

// POST /submit
// JSON body: { "first_name": "...", "last_name": "...", "email": "..." }
func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var in submissionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if in.FirstName == "" || in.LastName == "" || in.Email == "" {
		http.Error(w, "first_name, last_name, and email are required", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(
		`INSERT INTO submissions (first_name, last_name, email, exported)
         VALUES (?, ?, ?, 0)`,
		in.FirstName, in.LastName, in.Email,
	)
	if err != nil {
		log.Printf("insert error: %v", err)
		http.Error(w, "failed to save submission", http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	resp := map[string]interface{}{
		"id":         id,
		"first_name": in.FirstName,
		"last_name":  in.LastName,
		"email":      in.Email,
		"exported":   0,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /export?password=XXX&exported=0|1
func handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Password check (only if set)
	if exportPassword != "" {
		password := r.URL.Query().Get("password")
		if password == "" || password != exportPassword {
			w.Header().Set("WWW-Authenticate", `Basic realm="Export Protected"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	exportedParam := r.URL.Query().Get("exported")

	query := `SELECT id, first_name, last_name, email, exported, created_at FROM submissions`
	var args []interface{}

	if exportedParam == "0" || exportedParam == "1" {
		query += ` WHERE exported = ?`
		val, _ := strconv.Atoi(exportedParam)
		args = append(args, val)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("select error: %v", err)
		http.Error(w, "failed to query data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="submissions.csv"`)

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "first_name", "last_name", "email", "exported", "created_at"}); err != nil {
		log.Printf("csv header error: %v", err)
		return
	}

	for rows.Next() {
		var (
			id                         int
			firstName, lastName, email string
			exported                   int
			createdAt                  string
		)

		if err := rows.Scan(&id, &firstName, &lastName, &email, &exported, &createdAt); err != nil {
			log.Printf("scan error: %v", err)
			http.Error(w, "failed to read data", http.StatusInternalServerError)
			return
		}

		record := []string{
			strconv.Itoa(id),
			firstName,
			lastName,
			email,
			strconv.Itoa(exported),
			createdAt,
		}

		if err := cw.Write(record); err != nil {
			log.Printf("csv write error: %v", err)
			return
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Printf("csv flush error: %v", err)
	}
}
