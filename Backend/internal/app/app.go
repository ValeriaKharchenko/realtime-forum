package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"forum/internal/common"
	"forum/internal/user"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"net/http"
	"time"
)

type App struct {
	db          *sql.DB
	router      *http.ServeMux
	userService *user.Service
}

func (a *App) Run(port int, path string) error {
	db, err := sql.Open("sqlite3", "file:"+path+"?_foreign_keys=on")

	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	a.db = db
	common.InfoLogger.Println("Connect to db successfully")

	if err := a.createDB(); err != nil {
		return err
	}
	common.InfoLogger.Println("DataBase created successfully")

	a.router = http.NewServeMux()
	a.router.HandleFunc("/register", a.register)
	a.router.HandleFunc("/login", a.logIn)
	a.router.Handle("/logout", a.userIdentity(a.logOut))

	a.userService = user.NewService(a.db)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), a.router)
}

func (a *App) createDB() error {
	createDB, err := ioutil.ReadFile("./createTables.sql")
	if err != nil {
		return err
	}

	_, err = a.db.Exec(string(createDB))
	if err != nil {
		return err
	}

	return nil
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var u user.User
	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		common.InfoLogger.Println("Invalid json received from client")
		handleError(w, err)
		return
	}

	regU, err := a.userService.Register(u)
	if err != nil {
		common.ErrorLogger.Println(err.Error())
		handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(regU)
}

func (a *App) logIn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var loginReq struct {
		Credential string `json:"credential"`
		Password   string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&loginReq)
	if err != nil {
		handleError(w, err)
		return
	}

	code, err := a.userService.NewSession(loginReq.Credential, loginReq.Password)
	if err != nil {
		handleError(w, err)
		return
	}
	fmt.Printf(code)
	c := http.Cookie{
		Name:    "session",
		Value:   code,
		Expires: time.Now().AddDate(0, 0, 1),
		Path:    "/",
	}

	common.InfoLogger.Println("Setting cookie", c)
	http.SetCookie(w, &c)
	common.InfoLogger.Printf("%s logged in", loginReq.Credential)
}

func (a *App) logOut(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	values, _ := r.Context().Value("user").(userContext)
	fmt.Println(values)

	err := a.userService.LogOut(values.userID)
	if err != nil {
		handleError(w, err)
		return
	}

	common.InfoLogger.Printf("User %s logged out", values.login)
}

func handleError(w http.ResponseWriter, err error) {

	var appErr *common.AppError
	if errors.As(err, &appErr) {
		common.InfoLogger.Println(appErr.Message, ":", appErr.Err)
		w.WriteHeader(appErr.StatusCode)
		w.Write(appErr.Marshal())
		return
	}

	common.ErrorLogger.Println("Unhandled error occurred: ", err)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(common.SystemError(err).Marshal())
}
