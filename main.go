package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./game.db"
	}
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS scores (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			player_id INTEGER NOT NULL,
			score     INTEGER NOT NULL,
			played_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(player_id) REFERENCES players(id)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

// POST /api/players  {"name":"..."}
func handlePlayers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if len(name) > 50 {
		name = name[:50]
	}

	var id int64
	err := db.QueryRow(`SELECT id FROM players WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := db.Exec(`INSERT INTO players (name) VALUES (?)`, name)
		if err != nil {
			http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
			return
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "name": name})
}

// POST /api/scores  {"player_id":1,"score":42}
func handleScores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		PlayerID int64 `json:"player_id"`
		Score    int   `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlayerID == 0 {
		http.Error(w, `{"error":"player_id and score required"}`, http.StatusBadRequest)
		return
	}
	if _, err := db.Exec(`INSERT INTO scores (player_id, score) VALUES (?, ?)`, body.PlayerID, body.Score); err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// GET /api/leaderboard
func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := db.Query(`
		SELECT p.name, MAX(s.score) AS best_score, COUNT(s.id) AS games
		FROM scores s
		JOIN players p ON p.id = s.player_id
		GROUP BY s.player_id
		ORDER BY best_score DESC
		LIMIT 10
	`)
	if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Entry struct {
		Name      string `json:"name"`
		BestScore int    `json:"best_score"`
		Games     int    `json:"games"`
	}
	result := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Name, &e.BestScore, &e.Games); err == nil {
			result = append(result, e)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	initDB()

	http.HandleFunc("/api/players", handlePlayers)
	http.HandleFunc("/api/scores", handleScores)
	http.HandleFunc("/api/leaderboard", handleLeaderboard)
	http.Handle("/", http.FileServer(http.Dir(".")))

	log.Println("Server: http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
