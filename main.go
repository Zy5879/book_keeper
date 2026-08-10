package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)


type Book struct {
	Id string `json:"id"`
	Author string `json:"author"`
	Title string `json:"title"`
}

type BookHandler struct {
	db *pgxpool.Pool
}

func NewBookHandler(db *pgxpool.Pool) *BookHandler {
	return &BookHandler{db: db}
}

func (bh *BookHandler) AddBook(w http.ResponseWriter, r *http.Request) {
	var book Book;
	
	err := json.NewDecoder(r.Body).Decode(&book)
	if err != nil {
		log.Println("Error decoding: ", err)
		return
	}

	if book.Id == "" || book.Author == "" || book.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(
			 "Missing required fields in Book Struct",
		))
		return
	}

	bookJson, err := json.Marshal(book)
	if err != nil {
		log.Println("Error marshalling book")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	defer r.Body.Close()

	 row, err := bh.db.Query(r.Context(), "INSERT INTO books VALUES($1, $2, $3)", book.Id, book.Author, book.Title)
	 if err != nil {
		log.Println("error while inserting into database")
		return
	 }

	 log.Println("INSERTION COMPLETE")
	 defer row.Close()

	 w.WriteHeader(http.StatusCreated)
	 w.Write(bookJson)
	
}

func (bh *BookHandler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var deletedBook Book

	defer r.Body.Close()

	err := bh.db.QueryRow(
	r.Context(),
	`
		DELETE FROM books
		WHERE id = $1
		RETURNING id, author, title
	`,
	id,
	).Scan(
	&deletedBook.Id,
	&deletedBook.Author,
	&deletedBook.Title,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(fmt.Appendf(nil, "Failed to delete book: %s", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (bh *BookHandler) GetBooks(w http.ResponseWriter, r *http.Request) {
	var books []Book;

	rows,err := bh.db.Query(r.Context(), "SELECT * FROM books")
	if err != nil {
		log.Println("Error quering database: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	defer rows.Close()

	for rows.Next() {
		var book Book
		if err := rows.Scan(&book.Id, &book.Author, &book.Title); err != nil {
			return
		}
		books = append(books, book)
	}

	if err = rows.Err(); err != nil {
		return
	}

	booksJson, err := json.Marshal(books)
	if err != nil {
		log.Println("Error producing json: ", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(booksJson)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	databaseURL := os.Getenv("DATABASE_URL")

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}

	defer db.Close()

	bookHandler := NewBookHandler(db)
	
	
	err = db.Ping(context.Background()); if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	log.Println("Connected to DB")

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Route("/api/library", func(r chi.Router) {
		r.Get("/", bookHandler.GetBooks)
		r.Post("/", bookHandler.AddBook)
		r.Delete("/{id}", bookHandler.DeleteBook)
	})

	log.Printf("server starting at 8080")
	err = http.ListenAndServe(":8080", r); if err != nil {
		log.Fatal("Error starting server: ", err)
	}
}