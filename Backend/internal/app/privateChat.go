package app

import (
	"encoding/json"
	"fmt"
	"forum/internal/chat"
	"forum/internal/common"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/websocket"
)

type WSConnection struct {
	*websocket.Conn
}

type WSPayload struct {
	Action   string       `json:"action"`
	Message  string       `json:"message"`
	UserName string       `json:"user_name"`
	Receiver string       `json:"receiver"`
	Conn     WSConnection `json:"-"`
}

type JsonResponse struct {
	Action         string   `json:"action"`
	Message        string   `json:"message"`
	MessageType    string   `json:"message_type"`
	ConnectedUsers []string `json:"connected_users"`
}

func (a *App) userList(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	allUsers, err := a.chatService.FindAllUsers()
	if err != nil {
		handleError(w, err)
		return
	}
	common.InfoLogger.Println("Got list of users")

	if err := json.NewEncoder(w).Encode(allUsers); err != nil {
		handleError(w, err)
		return
	}
}

func (a *App) handleConnections(w http.ResponseWriter, r *http.Request) {
	val, _ := r.Context().Value("user").(userContext)
	login := val.login
	ws, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	fmt.Println("Client connected to endpoint")

	var msg JsonResponse
	msg.Message = `<em><small>Connected to Server</small></em>`
	conn := WSConnection{Conn: ws}
	//a.clients[conn] = ""
	//a.cl.Store("", conn)
	err = ws.WriteJSON(msg)
	if err != nil {
		log.Printf("error: %v", err)
	}
	go a.listenToWs(conn, login)
}

func (a *App) listenToWs(conn WSConnection, login string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()
	a.cl.Store(conn, login)
	a.sendListUsers()
	var payload WSPayload
	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			fmt.Println("cannot read JSON: ", err)
		} else {
			payload.Conn = conn
			payload.UserName = login
			a.wsChan <- payload
		}
	}
}

func (a *App) listenToWsChannel() {
	var response JsonResponse

	for {
		e := <-a.wsChan
		switch e.Action {
		//case "username":
		//	//a.clients[e.Conn] = e.UserName
		//	a.cl.Store(e.Conn, e.UserName)
		//	a.sendListUsers()

		case "left":
			//delete(a.clients, e.Conn)
			a.cl.Delete(e.Conn)
			a.sendListUsers()

		case "broadcast":
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s, %s", e.UserName, e.Receiver, e.Message)
			a.broadcastToAll(response)
		}
	}
}

func (a *App) sendListUsers() {
	var response JsonResponse
	users := a.getListOfUsers()
	response.Action = "list_users"
	response.ConnectedUsers = users
	a.broadcastToAll(response)
}

func (a *App) getListOfUsers() []string {
	var onlineUsers, userList []string
	//for _, x := range a.clients {
	//	if x != "" {
	//		onlineUsers = append(onlineUsers, x)
	//	}
	//}
	a.cl.Range(func(key, value interface{}) bool {
		if s, ok := value.(string); ok {
			onlineUsers = append(onlineUsers, s)
		}
		return true
	})
	sort.Sort(chat.StringSlice(onlineUsers))

	userList, err := a.chatService.FindAllUsers()
	if err != nil {
		fmt.Println(err)
		//handleError(w, err)
		//return nil
	}

	for _, u := range userList {
		if !inArray(u, onlineUsers) {
			onlineUsers = append(onlineUsers, u)
		}
	}
	return onlineUsers
}

func inArray(needle string, stack []string) bool {
	for _, el := range stack {
		if el == needle {
			return true
		}
	}
	return false
}

func (a *App) broadcastToAll(response JsonResponse) {
	//for client := range a.clients {
	//	err := client.WriteJSON(response)
	//	if err != nil {
	//		log.Println("websocket err")
	//		_ = client.Close()
	//		delete(a.clients, client)
	//	}
	//}
	a.cl.Range(func(key, value interface{}) bool {
		if conn, ok := key.(WSConnection); ok {
			if err := conn.WriteJSON(response); err != nil {
				common.WarningLogger.Println("websocket err:", err)
				_ = conn.Close()
				a.cl.Delete(value)
			}
		}
		return true
	})
}
