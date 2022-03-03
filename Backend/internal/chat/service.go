package chat

import (
	"database/sql"
	"forum/internal/common"
	"sort"
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
	sort.Strings(list)
	return list, nil
}
