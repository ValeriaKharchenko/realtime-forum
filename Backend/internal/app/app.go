package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"forum/internal/chat"
	"forum/internal/common"
	"forum/internal/post"
	"forum/internal/user"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	db          *sql.DB
	router      *http.ServeMux
	userService *user.Service
	postService *post.Service
	chatService *chat.Service
	upgrader    websocket.Upgrader
	ws          *chat.WS
}

func (a *App) Run(port int, path string) error {

	//DB initialisation and connection
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
	fmt.Println("Starting channel listener")

	a.router = http.NewServeMux()

	//user endpoints
	a.router.HandleFunc("/register", a.register)
	a.router.HandleFunc("/login", a.logIn)
	a.router.Handle("/logout", a.userIdentity(a.logOut))
	a.router.Handle("/profile", a.userIdentity(a.profile))
	a.router.Handle("/auth", a.userIdentity(a.auth))
	//a.router.Handle("/users", a.userIdentity(a.userList))

	//post endpoints
	a.router.Handle("/post/new", a.userIdentity(a.addPost))
	a.router.HandleFunc("/post/all", a.allPosts)
	a.router.Handle("/post/mark", a.userIdentity(a.addMark))
	a.router.HandleFunc("/post/by_category", a.findByCategory)
	a.router.Handle("/post/by_user", a.userIdentity(a.findByUser))
	a.router.Handle("/post/liked", a.userIdentity(a.findAllLiked))
	a.router.HandleFunc("/post/categories", a.allCategories)
	a.router.HandleFunc("/post", a.findByID)
	a.router.HandleFunc("/post/comments", a.findComments)

	//connection to file server
	fs := http.FileServer(http.Dir("../Frontend/app"))
	a.router.Handle("/", fs)
	a.router.Handle("/ws", a.userIdentity(a.handleConnections))

	a.router.Handle("/chat", a.userIdentity(a.getMessages))

	a.userService = user.NewService(a.db)
	a.postService = post.NewService(a.db)
	a.chatService = chat.NewService(a.db, a.userService)
	a.ws = chat.NewWS(a.userService, a.chatService)

	//ws
	a.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	common.InfoLogger.Println("Starting the application at port:", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), corsMW(a.router))
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

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

//User handlers

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

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

	if err := json.NewEncoder(w).Encode(regU); err != nil {
		handleError(w, err)
		return
	}
	a.ws.SendListUsers()
}

func (a *App) logIn(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

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
	setHeaders(w)

	values, _ := r.Context().Value("user").(userContext)
	fmt.Println(values)

	err := a.userService.LogOut(values.userID)
	if err != nil {
		handleError(w, err)
		return
	}

	common.InfoLogger.Printf("User %s logged out", values.login)
}

func (a *App) profile(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	common.InfoLogger.Println("Show personal info")

	val, _ := r.Context().Value("user").(userContext)
	data := val.userID
	var u user.User
	u, err := a.userService.FindUser(data)
	if err != nil {
		handleError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(u); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) auth(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	val, _ := r.Context().Value("user").(userContext)

	//setting values from context
	var u user.User
	u.ID = val.userID
	u.Email = val.email
	u.Login = val.login
	fmt.Println(u)

	if err := json.NewEncoder(w).Encode(u); err != nil {
		handleError(w, err)
		return
	}
}

//Post handlers

func (a *App) addPost(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	var postFromJson post.Post
	err := json.NewDecoder(r.Body).Decode(&postFromJson)
	if err != nil {
		handleError(w, err)
		return
	}

	// read from context
	u, _ := r.Context().Value("user").(userContext)

	postFromJson.UserId = u.userID

	newPost, err := a.postService.NewPost(postFromJson)
	if err != nil {
		handleError(w, err)
		return
	}

	common.InfoLogger.Println("New post added")
	if err := json.NewEncoder(w).Encode(newPost); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) allPosts(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	allPosts, err := a.postService.ShowAll()
	if err != nil {
		handleError(w, err)
		return
	}
	common.InfoLogger.Println("Get All Posts")

	if err := json.NewEncoder(w).Encode(allPosts); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) addMark(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	var markFromJson post.Mark
	err := json.NewDecoder(r.Body).Decode(&markFromJson)
	if err != nil {
		handleError(w, err)
		return
	}

	m, err := a.postService.AddMark(markFromJson)
	if err != nil {
		handleError(w, err)
		return
	}
	if m == nil {
		w.WriteHeader(http.StatusNoContent)
		common.InfoLogger.Println("Mark deleted")
	} else {
		w.WriteHeader(http.StatusOK)
		common.InfoLogger.Println("Mark added")
	}

	return
}

func (a *App) findByCategory(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	cat := r.URL.Query().Get("category_id")
	var posts []post.PostAndMarks

	if cat == "" {
		a.allPosts(w, r)
		return
	}

	id, err := strconv.Atoi(cat)
	if err != nil {
		handleError(w, err)
		return
	}
	posts, err = a.postService.FindByCategory(id)
	if err != nil {
		handleError(w, err)
		return
	}

	if len(posts) == 0 {
		common.InfoLogger.Println("No posts with that category")
	} else {
		common.InfoLogger.Println("Posts found")
	}
	if err := json.NewEncoder(w).Encode(posts); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) findByUser(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	u, _ := r.Context().Value("user").(userContext)
	fmt.Println("from context:", u)
	posts, err := a.postService.FindByUser(u.userID)
	if err != nil {
		handleError(w, err)
		return
	}
	common.InfoLogger.Println("Posts were found")
	if err := json.NewEncoder(w).Encode(posts); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) findAllLiked(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	// read from context
	u, _ := r.Context().Value("values").(userContext)
	fmt.Println("from context:", u)
	posts, err := a.postService.FindAllLiked(u.userID)
	if err != nil {
		handleError(w, err)
		return
	}
	if len(posts) == 0 {
		common.InfoLogger.Println("No liked posts")
	} else {
		common.InfoLogger.Println("Liked posts were found")
	}
	if err := json.NewEncoder(w).Encode(posts); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) allCategories(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	categories, err := a.postService.ShowAllCategories()
	if err != nil {
		handleError(w, err)
		return
	}

	common.InfoLogger.Println("Get All Categories")
	if err := json.NewEncoder(w).Encode(categories); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) findByID(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	id := r.URL.Query().Get("id")
	pID, err := strconv.Atoi(id)
	if err != nil {
		handleError(w, err)
		return
	}
	post, err := a.postService.FindById(pID)
	if err != nil {
		handleError(w, err)
		return
	}
	common.InfoLogger.Println("Post found")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) findComments(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)

	id := r.URL.Query().Get("id")
	pID, err := strconv.Atoi(id)
	if err != nil {
		handleError(w, err)
		return
	}

	comments, err := a.postService.CommentsByPostId(pID)
	if err != nil {
		handleError(w, err)
		return
	}
	if comments == nil {
		common.InfoLogger.Println("No comments")
		return
	}
	common.InfoLogger.Println("Comments found")
	if err := json.NewEncoder(w).Encode(comments); err != nil {
		handleError(w, err)
		return
	}
}

//Chat handlers

//func (a *App) userList(w http.ResponseWriter, r *http.Request) {
//	setHeaders(w)
//	allUsers, err := a.userService.FindAllUsers()
//	if err != nil {
//		handleError(w, err)
//		return
//	}
//	common.InfoLogger.Println("Got list of users")
//
//	if err := json.NewEncoder(w).Encode(allUsers); err != nil {
//		handleError(w, err)
//		return
//	}
//}

func (a *App) getMessages(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	val, _ := r.Context().Value("user").(userContext)

	//setting values from context
	sender := val.userID
	receiver := r.URL.Query().Get("with")
	skip := r.URL.Query().Get("skip")
	limit := r.URL.Query().Get("limit")
	intSkip, _ := strconv.Atoi(skip)
	intLimit, _ := strconv.Atoi(limit)

	messages, err := a.chatService.GetMessages(sender, receiver, intSkip, intLimit)
	if err != nil {
		handleError(w, err)
		return
	}
	common.InfoLogger.Printf("Got %d messages between %s and %s", len(messages), sender, receiver)

	if err := json.NewEncoder(w).Encode(messages); err != nil {
		handleError(w, err)
		return
	}
}

//WS handlers

func (a *App) handleConnections(w http.ResponseWriter, r *http.Request) {
	val, _ := r.Context().Value("user").(userContext)
	login := val.login
	ws, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	if err := a.ws.StartListener(ws, login); err != nil {
		handleError(w, err)
		return
	}
	common.InfoLogger.Println("Client connected to endpoint successfully")
}

//Error handler
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
