package chat

import (
	"fmt"
	"forum/internal/common"
	"forum/internal/user"
	"github.com/gorilla/websocket"
	"log"
	"sync"
	"time"
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
	Action         string       `json:"action"`
	Message        string       `json:"message"`
	NewMessage     Message      `json:"new_message"`
	Sender         string       `json:"sender"`
	ConnectedUsers []UserInChat `json:"connected_users"`
	Receiver       string       `json:"-"`
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
		ws.cl.Delete(login)
		ws.SendListUsers()
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()
	ws.cl.Store(login, conn)
	ws.SendListUsers()
	var payload WSPayload
	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			//fmt.Println("cannot read JSON: ", err)
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
			ws.SendListUsers()

		case "broadcast":
			if err := ws.chatService.SendMessage(e.UserName, e.Receiver, e.Message); err != nil {
				response.Action = "error"
				response.Message = fmt.Sprintf("Message was not save, DB error: %s", err)
				break
			}
			ws.SendListUsers()
			response.Action = "broadcast"

			var messages Message

			messages.Text = e.Message
			messages.To = e.Receiver
			messages.From = e.UserName
			messages.Data = time.Now()

			response.NewMessage = messages

			//response.Message = e.Message
			//response.Sender = e.UserName
			//response.Receiver = e.Receiver
			if ws.sendOne(response, e.UserName) {
				common.InfoLogger.Printf("Message sent to: %s\n", e.UserName)
			} else {
				common.InfoLogger.Printf("Cannot send a message to %s, no ws connection\n", e.UserName)
			}
			if ws.sendOne(response, e.Receiver) {
				common.InfoLogger.Printf("Message sent to: %s\n", e.Receiver)
			} else {
				common.InfoLogger.Printf("Cannot send a message to %s, no ws connection\n", e.Receiver)
			}
		}
	}
}

func (ws *WS) SendListUsers() {
	var response JsonResponse
	ws.cl.Range(func(key, value interface{}) bool {
		if login, ok := key.(string); ok {
			users := ws.getListOfUsers(login)
			response.Action = "list_users"
			response.ConnectedUsers = users
			//ws.broadcastToAll(response)
			ws.sendOne(response, login)
		}

		return true
	})
}

type UserInChat struct {
	UserLogin    string `json:"user_login"`
	OnlineStatus bool   `json:"online_status"`
	UserId       string `json:"user_id"`
}

func (ws *WS) getListOfUsers(login string) []UserInChat {
	var onlineUsers []UserInChat
	usersFromDB, err := ws.userService.FindAllUsers(login)
	if err != nil {
		fmt.Println(err)
	}

	for _, u := range usersFromDB {
		var us UserInChat
		us.UserLogin = u.Login
		us.UserId = u.ID
		_, us.OnlineStatus = ws.cl.Load(u.Login)
		fmt.Println("Status: ", us.UserLogin, us.OnlineStatus)
		onlineUsers = append(onlineUsers, us)
	}
	return onlineUsers
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
