package chat

import (
	"database/sql"
	"forum/internal/common"
	"sort"
	"strings"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) FindAllUsers() ([]string, error) {
	rows, err := s.db.Query(`SELECT login FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var s string
		err := rows.Scan(&s)
		if err != nil {
			common.InfoLogger.Println(err)
			continue
		}
		list = append(list, s)
	}
	sort.Sort(StringSlice(list))
	return list, nil
}

type StringSlice []string

func (x StringSlice) Len() int { return len(x) }

func (x StringSlice) Less(i, j int) bool { return strings.ToLower(x[i]) < strings.ToLower(x[j]) }
func (x StringSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (s *Service) SendMessage(sender, receiver, message string) error {
	if _, err := s.db.Exec(`INSERT INTO chat (msg_from, msg_to, msg) VALUES ($1, $2, $3)`, sender, receiver, message); err != nil {
		common.WarningLogger.Println("DB error: ", err)
		return err
	}
	return nil
}
