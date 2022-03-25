package chat

import (
	"database/sql"
	"fmt"
	"forum/internal/common"
	"forum/internal/user"
	"strings"
	"time"
)

type Service struct {
	db          *sql.DB
	userService *user.Service
}

func NewService(db *sql.DB, us *user.Service) *Service {
	return &Service{
		db:          db,
		userService: us,
	}
}

type Message struct {
	From string    `json:"msg_from"`
	To   string    `json:"msg_to"`
	Text string    `json:"msg_text"`
	Data time.Time `json:"data"`
}

type StringSlice []string

func (x StringSlice) Len() int { return len(x) }

func (x StringSlice) Less(i, j int) bool { return strings.ToLower(x[i]) < strings.ToLower(x[j]) }
func (x StringSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (s *Service) SendMessage(sender, receiver, message string) error {
	from, err := s.userService.FindByCredential(sender)
	if err != nil {
		return err
	}
	to, err := s.userService.FindByCredential(receiver)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO chat (msg_from, msg_to, msg) VALUES ($1, $2, $3)`, from.ID, to.ID, message); err != nil {
		common.WarningLogger.Println("DB error: ", err)
		return err
	}

	return nil
}

func (s *Service) GetMessages(sender, receiver string, skip, limit int) ([]Message, error) {
	rows, err := s.db.Query(`SELECT * FROM (
                  SELECT uf.login, ut.login, c.msg, c.send_at
                  FROM chat as c
                           JOIN users uf ON c.msg_from = uf.id
                           JOIN users ut ON c.msg_to = ut.id
                  WHERE c.msg_from = $1 AND c.msg_to = $2
                     OR c.msg_from = $2 AND c.msg_to = $1
                  ORDER BY send_at DESC
                  LIMIT $3, $4)
ORDER BY send_at ASC;`, sender, receiver, skip, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		err := rows.Scan(&m.From, &m.To, &m.Text, &m.Data)
		if err != nil {
			common.InfoLogger.Println(err)
			continue
		}
		messages = append(messages, m)
	}
	fmt.Println(messages)
	return messages, nil
}
