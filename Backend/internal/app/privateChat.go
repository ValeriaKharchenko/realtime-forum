package app

import (
	"encoding/json"
	"fmt"
	"forum/internal/common"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sort"
)

type WSConnection struct {
	*websocket.Conn
}

type WSPayload struct {
	Action   string       `json:"action"`
	Message  string       `json:"message"`
	UserName string       `json:"user_name"`
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
	ws, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	fmt.Println("Client connected to endpoint")

	var msg JsonResponse
	msg.Message = `<em><small>Connected to Server</small></em>`
	conn := WSConnection{Conn: ws}
	a.clients[conn] = ""
	err = ws.WriteJSON(msg)
	if err != nil {
		log.Printf("error: %v", err)
	}
	go a.listenToWs(&conn)
}

func (a *App) listenToWs(conn *WSConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()
	var payload WSPayload
	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			fmt.Println("cannot read JSON: ", err)
		} else {
			fmt.Printf("%#v", payload)
			payload.Conn = *conn
			a.wsChan <- payload
		}
	}
}

func (a *App) listenToWsChannel() {
	var response JsonResponse

	for {
		e := <-a.wsChan
		switch e.Action {
		case "username":
			a.clients[e.Conn] = e.UserName
			users := a.getListOfUsers()
			response.Action = "list_users"
			response.ConnectedUsers = users
			a.broadcastToAll(response)

		case "left":
			response.Action = "list_users"
			delete(a.clients, e.Conn)
			users := a.getListOfUsers()
			response.ConnectedUsers = users
			a.broadcastToAll(response)

		case "broadcast":
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s", e.UserName, e.Message)
			a.broadcastToAll(response)
		}
	}
}

func (a *App) getListOfUsers() []string {
	var userList []string
	for _, x := range a.clients {
		if x != "" {
			userList = append(userList, x)
		}
	}
	sort.Strings(userList)
	return userList
}

func (a *App) broadcastToAll(response JsonResponse) {
	for client := range a.clients {
		err := client.WriteJSON(response)
		if err != nil {
			log.Println("websocket err")
			_ = client.Close()
			delete(a.clients, client)
		}
	}
}
