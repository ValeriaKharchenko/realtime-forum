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
	Action  string `json:"action"`
	Message string `json:"message"`
	//MessageType    string   `json:"message_type"`
	ConnectedUsers []string `json:"connected_users"`
	Receiver       string   `json:"-"`
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
	a.cl.Store(login, conn)
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

		case "left":
			a.cl.Delete(e.UserName)
			a.sendListUsers()

		case "broadcast":
			if err := a.chatService.SendMessage(e.UserName, e.Receiver, e.Message); err != nil {
				response.Action = "error"
				response.Message = fmt.Sprintf("Message was not save, DB error: %s", err)
				break
			}
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s", e.UserName, e.Message)
			response.Receiver = e.Receiver
			if a.sendOne(response, e.UserName) && a.sendOne(response, e.Receiver) {
				common.InfoLogger.Println("Personal message sent")
			} else {
				common.InfoLogger.Printf("Cannot send a message from %s to %s\n", e.UserName, e.Receiver)
			}
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
	a.cl.Range(func(key, value interface{}) bool {
		if s, ok := key.(string); ok {
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
	a.cl.Range(func(key, value interface{}) bool {
		a.sendOne(response, key.(string))
		return true
	})
}

func (a *App) sendOne(response JsonResponse, sendTo string) bool {
	if conn, ok := a.cl.Load(sendTo); ok {
		c := conn.(WSConnection)
		if err := c.WriteJSON(response); err != nil {
			common.WarningLogger.Println("websocket err:", err)
			_ = c.Close()
			a.cl.Delete(sendTo)
			return false
		}
		return true
	}
	return false
}
