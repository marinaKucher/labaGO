package main

import (
	"database/sql"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gocraft/web"
	_ "github.com/nakagami/firebirdsql"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

var mutex sync.RWMutex

var cache = make(map[int]*Document)

var db *sql.DB

//Context
type Context struct {
}

type Document struct {
	Name string
	Data []byte
}

func (c *Context) renderHomePage(rw web.ResponseWriter, req *web.Request) {
	rw.Header().Set("Location", "/docs")
	rw.WriteHeader(http.StatusTemporaryRedirect)
}

func (c *Context) getDocList(rw web.ResponseWriter, req *web.Request) {
	log.Println(0)
	rows, err := db.Query("SELECT id, name FROM docs;")
	if err != nil {
		log.Println(1)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	defer rows.Close()

	names := make(map[int]string)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Println(5)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
		names[id] = name
	}

	err = templates.ExecuteTemplate(rw, "list.html", names)
	if err != nil {
		log.Println(2)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func renderDoc(rw web.ResponseWriter, title, data string) {
	err := templates.ExecuteTemplate(rw, "doc.html", struct {
		Title string
		Data  string
	}{
		title,
		data})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) getDoc(rw web.ResponseWriter, req *web.Request) {
	id, err := strconv.Atoi(req.PathParams["id"])
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	mutex.RLock()
	value, isCached := cache[id]
	mutex.RUnlock()

	if isCached {
		renderDoc(rw, value.Name, string(value.Data))
	} else {
		result := db.QueryRow("SELECT name, data FROM docs WHERE id=$1;", id)

		var name string
		var data []byte
		err = result.Scan(&name, &data)
		if err != nil {
			if err == sql.ErrNoRows {
				renderDoc(rw, "Такого документа не существует", "Извините :(")
			}
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
		mutex.Lock()
		cache[id] = &Document{Name: name,
			Data: data}
		mutex.Unlock()

		renderDoc(rw, name, string(data))
	}
}

func (c *Context) deleteDoc(rw web.ResponseWriter, req *web.Request) {
	id, err := strconv.Atoi(req.PathParams["id"])
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	mutex.RLock()
	_, isCached := cache[id]
	mutex.RUnlock()

	if isCached {
		mutex.Lock()
		delete(cache, id)
		mutex.Unlock()
	}

	_, err = db.Exec("DELETE FROM docs WHERE id=$1;", id)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	rw.Header().Set("Location", "/docs")
	rw.WriteHeader(http.StatusMovedPermanently)
}

func (c *Context) sendDocForm(rw web.ResponseWriter, req *web.Request) {
	err := templates.ExecuteTemplate(rw, "send.html", c)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) sendDoc(rw web.ResponseWriter, req *web.Request) {
	file, _, err := req.FormFile("file_data")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	_, err = db.Exec("INSERT INTO docs(name, data) VALUES($1, $2);", req.FormValue("file_name"), data)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	rw.Header().Set("Location", "/docs")
	rw.WriteHeader(http.StatusMovedPermanently)
}

func main() {
	connStr := "SYSDBA:masterkey@LOCALHOST:27015/C:\\Users\\User\\Desktop\\BD\\GOLAB.fdb"
	d, err := sql.Open("firebirdsql", connStr)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db = d

	rootRouter := web.New(Context{}).
		Middleware(web.LoggerMiddleware).
		Middleware(web.ShowErrorsMiddleware).
		Get("/", (*Context).renderHomePage).
		Get("/docs", (*Context).getDocList).
		Get("/docs/:id", (*Context).getDoc).
		Get("/send", (*Context).sendDocForm).
		Post("/send", (*Context).sendDoc).
		Get("/delete/:id", (*Context).deleteDoc)

	panic(http.ListenAndServe("localhost:3000", rootRouter))
}
