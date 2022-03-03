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
	sort.Sort(stringSlice(list))
	return list, nil
}

type stringSlice []string

func (x stringSlice) Len() int           { return len(x) }
func (x stringSlice) Less(i, j int) bool { return strings.ToLower(x[i]) < strings.ToLower(x[j]) }
func (x stringSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
