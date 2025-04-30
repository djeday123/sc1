package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/djeday123/sc1/pkg/playwright"
	"github.com/go-sql-driver/mysql"
)

// SetupDatabase establishes connection to the database
func SetupDatabase() (*sql.DB, error) {
	// Get connection parameters from environment variables
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "3306" // Default MySQL port
	}

	// MySQL configuration
	cfg := mysql.Config{
		User:                 dbUser,
		Passwd:               dbPassword,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%s", dbHost, dbPort),
		DBName:               dbName,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	// Open connection
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// CreateNewsTable creates news table if it doesn't exist
func CreateNewsTable(db *sql.DB) error {
	query := `CREATE TABLE IF NOT EXISTS news (
		id INT AUTO_INCREMENT PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		url VARCHAR(255) NOT NULL,
		source VARCHAR(100),
		timestamp DATETIME,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	_, err := db.Exec(query)
	return err
}

// SaveNewsToDB saves news items to database
func SaveNewsToDB(db *sql.DB, news []playwright.NewsItem) error {
	query := `INSERT INTO news (title, description, url, source, timestamp) VALUES (?, ?, ?, ?, ?)`

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Prepare statement
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Insert news items
	for _, item := range news {
		_, err = stmt.Exec(item.Title, item.Description, item.URL, item.Source, time.Now())
		if err != nil {
			return err
		}
	}

	// Commit transaction
	return tx.Commit()
}
