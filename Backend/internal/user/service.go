package user

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"forum/internal/common"
	"github.com/mattn/go-sqlite3"
	uuid "github.com/satori/go.uuid"
	"strings"
	"time"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) Register(user User) (User, error) {
	if err := validateUser(user); err != nil {
		return User{}, err
	}
	user.generateID()
	user.hashPassword()
	if err := s.userToDB(user); err != nil {
		return User{}, err
	}
	common.InfoLogger.Println("New user was added to DB")
	user.cleanUp()

	return user, nil
}

var (
	userCol    = "id, email, login, password, age, gender, first_name, last_name"
	sessionCol = "session_key, user_id, expired_at"
)

func (s *Service) userToDB(user User) error {
	query := fmt.Sprintf("INSERT INTO users (%s) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", userCol)
	if _, err := s.db.Exec(query, user.ID, user.Email, user.Login, user.Password, user.Age, user.Gender, user.FirstName, user.LastName); err != nil {
		common.ErrorLogger.Println(err)
		var sErr sqlite3.Error
		if errors.As(err, &sErr) {
			if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
				return common.InvalidArgumentError(nil, "user with this email already exists")
			}
			if strings.Contains(err.Error(), "UNIQUE constraint failed: users.login") {
				return common.InvalidArgumentError(nil, "user with this login already exists")
			}
		}
		return common.SystemError(err)
	}
	return nil
}

func (s *Service) NewSession(str, pwd string) (string, error) {

	u, err := s.FindByCredential(str)
	if err != nil {
		return "", err
	}
	if !u.comparePassword(u.Password, pwd) {
		return "", common.InvalidArgumentError(nil, "password is incorrect")
	}
	sessionID := generateCookieCode()
	if err := s.createSession(u.ID, sessionID); err != nil {
		return "", err
	}
	//if err := s.UpdateStatus(u.ID); err != nil {
	//	return "", err
	//}
	//fmt.Println(u.Login, " is online")
	return sessionID + "|" + u.ID, nil
}

func (s *Service) FindByCredential(str string) (User, error) {
	query := fmt.Sprintf("SELECT %s FROM users WHERE login=$1 OR email=$1 OR id=$1", userCol)
	row := s.db.QueryRow(query, str)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Login, &u.Password, &u.Age, &u.Gender, &u.FirstName, &u.LastName)
	if err != nil {
		return User{}, common.NotFoundError(nil, "cannot find user with this login")
	}

	return u, nil
}

func (s *Service) createSession(userID, sessionID string) error {
	query := fmt.Sprintf("INSERT OR REPLACE INTO sessions (%s) VALUES ($1, $2, $3)", sessionCol)
	t := time.Now().Add(24 * time.Hour)
	_, err := s.db.Exec(query, sessionID, userID, t)

	if err != nil {
		//// update session_key if exist
		//if err.Error() == "UNIQUE constraint failed: sessions.user_id" {
		//	query := fmt.Sprintf("UPDATE sessions SET session_key=$1 WHERE user_id=$2")
		//	t := time.Now().Add(24 * time.Hour)
		//	_, err := s.db.Exec(query, sessionID, userID, t)
		//	if err != nil {
		//		return common.SystemError(err)
		//}
		//return nil
		//}
		return common.SystemError(err)
	}
	return nil
}

func generateCookieCode() string {
	h := hmac.New(sha256.New, []byte("Hello tere privet"))
	newID := uuid.NewV4().Bytes()
	h.Write(newID)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *Service) CheckSession(key, userID string) (User, error) {
	query := fmt.Sprintf("SELECT u.id, u.email, u.login FROM sessions INNER JOIN users u on u.id = sessions.user_id WHERE session_key=$1 AND user_id=$2")
	row := s.db.QueryRow(query, key, userID)

	var user User
	err := row.Scan(&user.ID, &user.Email, &user.Login)
	if err != nil {
		common.InfoLogger.Println(err)
		return User{}, common.InvalidArgumentError(err, "no current session")
	}

	return user, nil
}

func (s *Service) LogOut(userID string) error {
	query := fmt.Sprintf("DELETE FROM sessions WHERE user_id=$1")
	_, err := s.db.Exec(query, userID)
	if err != nil {
		return common.InvalidArgumentError(err, "no current session")
	}
	return nil
}

//func (s *Service) LogOut(userID string) error {
//	tx, err := s.db.Begin()
//	if err != nil {
//		return common.DataBaseError(err)
//	}
//	query := fmt.Sprintf("DELETE FROM sessions WHERE user_id=$1")
//	_, err = tx.Exec(query, userID)
//	if err != nil {
//		if err := tx.Rollback(); err != nil {
//			common.ErrorLogger.Println(err)
//			return common.DataBaseError(err)
//		}
//		return common.InvalidArgumentError(err, "no current session")
//	}
//	query = fmt.Sprintf("DELETE FROM online_status WHERE user_id=$1")
//	_, err = tx.Exec(query, userID)
//	if err != nil {
//		if err := tx.Rollback(); err != nil {
//			common.ErrorLogger.Println(err)
//			return common.DataBaseError(err)
//		}
//		return common.InvalidArgumentError(err, "user is not online")
//	}
//	if err := tx.Commit(); err != nil {
//		common.ErrorLogger.Println(err)
//		return common.DataBaseError(err)
//	}
//	return nil
//}

func (s *Service) FindUser(id string) (User, error) {
	var u User
	u, err := s.FindByCredential(id)
	if err != nil {
		return User{}, err
	}
	u.cleanUp()
	return u, nil
}

//func (s *Service) UpdateStatus(userID string) error {
//	t := time.Now().Add(5 * time.Minute)
//	query := fmt.Sprintf(`insert or replace into online_status (user_id, expires_at) values ($1, $2)`)
//	_, err := s.db.Exec(query, userID, t)
//	if err != nil {
//		return common.SystemError(err)
//	}
//	return nil
//}

func (s *Service) FindAllUsers(login string) ([]string, error) {
	user, err := s.FindByCredential(login)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`select u.login from users u
  left outer join chat c on (u.id = msg_from OR u.id = c.msg_to) AND (c.msg_from=$1 or c.msg_to=$1)
where u.id <> $1
group by u.login
ORDER BY max(c.send_at) DESC, u.login COLLATE NOCASE ASC;`, user.ID)
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
	return list, nil
}
