package chat

import (
	"fmt"
	"forum/internal/common"
	"forum/internal/user"
	"github.com/gorilla/websocket"
	"log"
	"sort"
	"sync"
)

type WS struct {
	cl          sync.Map
	wsChan      chan WSPayload
	userService *user.Service
	chatService *Service
}

func NewWS(uService *user.Service, cS *Service) *WS {
	w := &WS{}
	w.wsChan = make(chan WSPayload)
	w.userService = uService
	w.chatService = cS
	go w.listenToWsChannel()
	return w
}

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
	ConnectedUsers []string `json:"connected_users"`
	Receiver       string   `json:"-"`
}

func (ws *WS) StartListener(webS *websocket.Conn, userLogin string) error {
	var msg JsonResponse
	msg.Message = `<em><small>Connected to Server</small></em>`

	conn := WSConnection{Conn: webS}
	err := webS.WriteJSON(msg)
	if err != nil {
		return err
	}
	go ws.listenToWs(conn, userLogin)
	return nil
}

func (ws *WS) listenToWs(conn WSConnection, login string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()
	ws.cl.Store(login, conn)
	ws.sendListUsers()
	var payload WSPayload
	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			fmt.Println("cannot read JSON: ", err)
		} else {
			payload.Conn = conn
			payload.UserName = login
			ws.wsChan <- payload
		}
	}
}

func (ws *WS) listenToWsChannel() {
	var response JsonResponse

	for {
		e := <-ws.wsChan
		switch e.Action {

		case "left":
			ws.cl.Delete(e.UserName)
			ws.sendListUsers()

		case "broadcast":
			if err := ws.chatService.SendMessage(e.UserName, e.Receiver, e.Message); err != nil {
				response.Action = "error"
				response.Message = fmt.Sprintf("Message was not save, DB error: %s", err)
				break
			}
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s", e.UserName, e.Message)
			response.Receiver = e.Receiver
			if ws.sendOne(response, e.UserName) && ws.sendOne(response, e.Receiver) {
				common.InfoLogger.Println("Personal message sent")
			} else {
				common.InfoLogger.Printf("Cannot send a message from %s to %s\n", e.UserName, e.Receiver)
			}
		}
	}
}

func (ws *WS) sendListUsers() {
	var response JsonResponse
	users := ws.getListOfUsers()
	response.Action = "list_users"
	response.ConnectedUsers = users
	ws.broadcastToAll(response)
}

func (ws *WS) getListOfUsers() []string {
	var onlineUsers, userList []string
	ws.cl.Range(func(key, value interface{}) bool {
		if s, ok := key.(string); ok {
			onlineUsers = append(onlineUsers, s)
		}
		return true
	})
	sort.Sort(StringSlice(onlineUsers)) // сделать приватной

	userList, err := ws.userService.FindAllUsers()
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

func (ws *WS) broadcastToAll(response JsonResponse) {
	ws.cl.Range(func(key, value interface{}) bool {
		ws.sendOne(response, key.(string))
		return true
	})
}

func (ws *WS) sendOne(response JsonResponse, sendTo string) bool {
	if conn, ok := ws.cl.Load(sendTo); ok {
		c := conn.(WSConnection)
		if err := c.WriteJSON(response); err != nil {
			common.WarningLogger.Println("websocket err:", err)
			_ = c.Close()
			ws.cl.Delete(sendTo)
			return false
		}
		return true
	}
	return false
}
