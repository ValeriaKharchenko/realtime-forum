package app

import (
	"encoding/json"
	"forum/internal/common"
	"net/http"
)

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
