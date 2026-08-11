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


type IsCompleted string

const (
	Active IsCompleted = "active"
	Finished IsCompleted = "finished"
)


type Book struct {
	Id string `json:"id"`
	Author string `json:"author"`
	Title string `json:"title"`
	Status IsCompleted `json:"status"`
}

type BookUpdateReq struct {
	Status *IsCompleted `json:"status"`
}

type BookHandler struct {
	db *pgxpool.Pool
}

func NewBookHandler(db *pgxpool.Pool) *BookHandler {
	return &BookHandler{db: db}
}

func (b IsCompleted) Valid() bool {
	switch b {
	case Active, Finished:
		return true
	default:
		return false
	
	}
}

func (bh *BookHandler) AddBook(w http.ResponseWriter, r *http.Request) {
	var book Book;
	
	err := json.NewDecoder(r.Body).Decode(&book)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(fmt.Appendf(nil, "unable to decode request body: %s", err))
		return
	}

	if book.Id == "" || book.Author == "" || book.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(
			 "Missing required fields in Book Struct",
		))
		return
	}

	if book.Status.Valid() == false {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(fmt.Appendf(nil, "status not vaild: %s", book.Status))
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

	 row, err := bh.db.Query(r.Context(), "INSERT INTO books VALUES($1, $2, $3, $4)", book.Id, book.Author, book.Title, book.Status)
	 if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write(fmt.Appendf(nil, "Unable to find row: %s" ,err))
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
		RETURNING id, author, title, status
	`,
	id,
	).Scan(
	&deletedBook.Id,
	&deletedBook.Author,
	&deletedBook.Title,
	&deletedBook.Status,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(fmt.Appendf(nil, "Failed to delete book: %s", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (bh *BookHandler) UpdateBookStatusById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updateBook Book
	var req BookUpdateReq

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if req.Status == nil  {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No Status sent in request"))
		return
	}

	if !(req.Status.Valid()) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("Status does not match expected type"))
		return

	}

	err = bh.db.QueryRow(r.Context(), "UPDATE books SET status = $1 WHERE id = $2 RETURNING id, author, title, status;",*req.Status, id).Scan(
		&updateBook.Id,
		&updateBook.Author,
		&updateBook.Title,
		&updateBook.Status,
	);

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(fmt.Appendf(nil, "Failed to update book by id: %s, %s",id, err))
		return
	}

	log.Println("TABLE UPDATED WITH NEW STATUS")

	bookJson, err := json.Marshal(updateBook)
	if err != nil {
		log.Println("Error marshalling book")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(bookJson)
}

func (bh *BookHandler) GetBookById(w http.ResponseWriter, r *http.Request) {
	var book Book;
	id := chi.URLParam(r, "id")

	err := bh.db.QueryRow(r.Context(), "SELECT * FROM books WHERE id = $1;", id).Scan(
		&book.Id,
		&book.Author,
		&book.Title,
		&book.Status,
	)
	if err != nil {
		log.Println("Cannot find row")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(fmt.Appendf(nil, "Cannot find row with id: %s, err: %s", id, err))
		return
	}

	booksJson, err := json.Marshal(book)
	if err != nil {
		log.Println("Error producing json: ", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(booksJson)
	w.WriteHeader(http.StatusFound)
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
		if err := rows.Scan(&book.Id, &book.Author, &book.Title, &book.Status); err != nil {
			return
		}
		books = append(books, book)
	}

	if err = rows.Err(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write(fmt.Appendf(nil, "Unable to find row: %s" , err))
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
		r.Get("/{id}", bookHandler.GetBookById)
		r.Post("/", bookHandler.AddBook)
		r.Delete("/{id}", bookHandler.DeleteBook)
		r.Patch("/{id}",bookHandler.UpdateBookStatusById)
	})

	log.Printf("server starting at 8080")
	err = http.ListenAndServe(":8080", r); if err != nil {
		log.Fatal("Error starting server: ", err)
	}
}