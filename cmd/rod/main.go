package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/djeday123/sc1/pkg/database"
	"github.com/djeday123/sc1/pkg/rod"
	"github.com/joho/godotenv"
)

func main() {
	// Enable detailed logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Application starting...")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Failed to load .env file, using environment variables")
	}

	// Get search query from command line arguments or use default
	searchQuery := "RTErdogan"
	if len(os.Args) > 1 {
		searchQuery = os.Args[1]
	}
	log.Printf("Search query: %s", searchQuery)

	// Check proxy settings
	proxyServer := os.Getenv("PROXY_SERVER")
	if proxyServer != "" {
		log.Printf("Using proxy: %s", proxyServer)
		if os.Getenv("PROXY_USER") != "" {
			log.Printf("Proxy with auth configured: %s", os.Getenv("PROXY_USER"))
		}
	} else {
		log.Println("No proxy configured, using direct connection")
	}

	// Check headless mode
	headless := os.Getenv("HEADLESS") != "false"
	if !headless {
		log.Println("Running in browser visible mode (headless=false)")
	} else {
		log.Println("Running in browser hidden mode (headless=true)")
	}

	// Determine if we should skip database operations
	skipDB := os.Getenv("SKIP_DB") == "true"
	var db *sql.DB
	var err error

	if !skipDB {
		// Connect to database
		db, err = database.SetupDatabase()
		if err != nil {
			log.Fatalf("Database connection error: %v", err)
		}
		defer db.Close()
		log.Println("Database connection established")

		// Create table if it doesn't exist
		if err := database.CreateNewsTable(db); err != nil {
			log.Fatalf("Error creating table: %v", err)
		}
		log.Println("News table ready")
	} else {
		log.Println("Database-less mode. Data will be output to console.")
	}

	// Опционально: проверка IP адреса
	// log.Println("Checking IP address...")
	// ipInfo, err := rod.CheckIPAddress()
	// if err != nil {
	//     log.Fatalf("Error checking IP: %v", err)
	// }
	// log.Printf("Current IP information: %s", ipInfo)

	// Get news from Google
	log.Println("Starting Google News scraping...")
	news, err := rod.ScrapeGoogleNews(searchQuery)
	if err != nil {
		log.Fatalf("Error scraping news: %v", err)
	}
	log.Printf("Found %d news items", len(news))

	// Output news to console
	for i, item := range news {
		fmt.Printf("%d. %s\n", i+1, item.Title)
		fmt.Printf("   Source: %s\n", item.Source)
		fmt.Printf("   URL: %s\n", item.URL)
		fmt.Printf("   Description: %s\n\n", item.Description)
	}

	log.Println("Application completed successfully")
}
