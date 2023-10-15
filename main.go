package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/libsql/libsql-client-go/libsql"
)

type Todo struct {
	Content  string
	Finished bool
	Id       int
}

type TodoList struct {
	Todos []*Todo
	db    *sql.DB
}

func NewTodoList(db *sql.DB) (*TodoList, error) {
	todos := []*Todo{}
	rows, err := db.Query("SELECT * FROM todos;")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var todo Todo

		err := rows.Scan(&todo.Id, &todo.Content, &todo.Finished)
		if err != nil {
			return nil, err
		}

		todos = append(todos, &todo)
	}

	todoList := &TodoList{
		Todos: todos,
		db:    db,
	}

	return todoList, nil
}

func (todoList *TodoList) FindTodo(todoId int) (*Todo, error) {
	for id := 1; id <= len(todoList.Todos); id++ {
		if todoId == id {
			return todoList.Todos[id-1], nil
		}
	}

	return nil, errors.New("cannot find todo with the provided id")
}

func (todoList *TodoList) NewTodo(content string) (*Todo, error) {
	_, err := todoList.db.Exec(
		fmt.Sprintf(`INSERT INTO todos (finished, content) VALUES (0, "%s")`, content),
	)
	if err != nil {
		return nil, err
	}

	todo := &Todo{
		Id:       len(todoList.Todos) + 1,
		Content:  content,
		Finished: false,
	}

	todoList.Todos = append(todoList.Todos, todo)

	return todo, nil
}

func (todoList *TodoList) UpdateTodo(newTodo *Todo) error {
	if len(todoList.Todos) > newTodo.Id {
		return errors.New("todo id not found")
	}

	todoFinished := int8(0)

	if newTodo.Finished {
		todoFinished = 1
	}

	_, err := todoList.db.Exec(
		fmt.Sprintf(
			`UPDATE todos SET finished=%b, content="%s" WHERE id=%d`,
			todoFinished,
			newTodo.Content,
			newTodo.Id,
		),
	)
	if err != nil {
		panic(err)
	}

	todoList.Todos[newTodo.Id-1] = newTodo

	return nil
}

func InitDb(url string) (*sql.DB, error) {
	db, err := sql.Open("libsql", url)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id integer PRIMARY KEY AUTOINCREMENT NOT NULL,
		content text NOT NULL,
		finished integer DEFAULT false NOT NULL
	)`)

	if err != nil {
		return nil, err
	}

	return db, nil
}

func HandleIndex(todos *TodoList) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		tmpl := template.Must(template.ParseFiles("index.html"))
		err := tmpl.Execute(w, todos)
		if err != nil {
			panic(err)
		}
	}
}

func HandleCreateTodo(todos *TodoList) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		content := r.PostFormValue("content")
		tmpl := template.Must(template.ParseFiles("index.html"))
		todo, err := todos.NewTodo(content)
		if err != nil {
			panic(err)
		}

		err = tmpl.ExecuteTemplate(w, "todo", todo)
		if err != nil {
			panic(err)
		}
	}
}

func HandleToggleTodo(todos *TodoList) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		todoId, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			panic(err)
		}

		todo, err := todos.FindTodo(todoId)
		if err != nil {
			panic(err)
		}

		tmpl := template.Must(template.ParseFiles("index.html"))
		todo.Finished = !todo.Finished

		todos.UpdateTodo(todo)
		tmpl.ExecuteTemplate(w, "todo", todo)
	}
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		panic(err)
	}

	db, err := InitDb(
		fmt.Sprintf("%s?authToken=%s", os.Getenv("DATABASE_URL"), os.Getenv("DATABASE_AUTH_TOKEN")),
	)
	if err != nil {
		panic(err)
	}

	todos, err := NewTodoList(db)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", HandleIndex(todos))
	http.HandleFunc("/todo", HandleCreateTodo(todos))
	http.HandleFunc("/todo/toggle", HandleToggleTodo(todos))

	log.Fatal(http.ListenAndServe(":3000", nil))
}
