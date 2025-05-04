package database

import (
	"context"
	"crypto_websocket/internal/config"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/mattn/go-sqlite3"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

type SQLiteDB struct {
	DB *sql.DB
}

type Databases struct {
	Postgres *PostgresDB
	Sqlite   *SQLiteDB
}

// function to initilize database connection
func InitDB(cfg map[string]*config.DatabaseConfig) (*Databases, error) {
	pst, err := InitPostgresDB(cfg["postgres"], "public")
	if err != nil {
		return nil, fmt.Errorf("error initializing postgres: %v", err)
	}

	sqlite, err := InitSQLite(cfg["sqlite3"])
	if err != nil {
		return nil, fmt.Errorf("error initializing sqlite: %v", err)
	}

	return &Databases{
		Postgres: pst,
		Sqlite:   sqlite,
	}, nil
}

// function to initialize postgres
func InitPostgresDB(cfg *config.DatabaseConfig, schema string) (*PostgresDB, error) {
	connConfig := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DbName, cfg.SslMode,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// test the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	// Let's initialize our needed tables
	// frist drop table order_book everytime we start the app
	query_drop := fmt.Sprintf(`DROP TABLE IF EXISTS %s.order_book;`, schema)
	_, err = pool.Exec(ctx, query_drop)
	if err != nil {
		return nil, fmt.Errorf("failed to drop order_book table: %w", err)
	}

	// now let's create the order_book table
	query_order_book := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_book(
		id SERIAL PRIMARY KEY,
		exchange_name VARCHAR(50) NOT NULL,
		pair_symbol VARCHAR(25) NOT NULL,
		order_position VARCHAR(10) NOT NULL,
		price DOUBLE PRECISION NOT NULL,
		volume DOUBLE PRECISION NOT NULL,
		created_datetime TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`, schema)
	_, err = pool.Exec(ctx, query_order_book)
	if err != nil {
		return nil, fmt.Errorf("failed to create order_book table: %w", err)
	}

	query_trade_activity := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.trade_activity (
			id SERIAL PRIMARY KEY,
			exchange_name VARCHAR(50) NOT NULL,
			pair_symbol VARCHAR(25) NOT NULL,
			order_type VARCHAR(10) NOT NULL,
			price DOUBLE PRECISION NOT NULL,
			volume DOUBLE PRECISION NOT NULL,
			created_datetime TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, schema)
	_, err = pool.Exec(ctx, query_trade_activity)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade_activity table: %v", err)
	}

	return &PostgresDB{pool: pool}, nil
}

func InitSQLite(cfg *config.DatabaseConfig) (*SQLiteDB, error) {
	sqlite, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer sqlite.Close()

	// let's initialize needed tables
	// first initialize our orderbook
	// we need to ensure that order_book is clean up everytime
	_, err = sqlite.Exec("DROP TABLE IF EXISTS order_book;")
	if err != nil {
		return nil, fmt.Errorf("failed to drop order_book table; %w", err)
	}

	// create the order_book
	_, err = sqlite.Exec(`CREATE TABLE IF NOT EXISTS order_book (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		exchange_name TEXT NOT NULL,
		pair_symbol TEXT NOT NULL,
		order_type TEXT NOT NULL,
		price REAL NOT NULL,
		volume REAL NOT NULL,
		created_datetime TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, fmt.Errorf("unable to create table order_book: %w", err)
	}

	// create trade_activity table
	_, err = sqlite.Exec(`CREATE TABLE IF NOT EXISTS trade_activity (
		id INTEGER PRIMARY	KEY AUTOINCREMENT,
		exchange_name TEXT NOT NULL,
		pair_symbol TEXT NOT NULL,
		order_type TEXT,
		price REAL NOT NULL,
		volume REAL NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade_activity table: %w", err)
	}

	return &SQLiteDB{
		DB: sqlite,
	}, nil
}
