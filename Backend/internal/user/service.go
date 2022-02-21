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

	u, err := s.findByCredential(str)
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
	return sessionID + "|" + u.ID, nil
}

func (s *Service) findByCredential(str string) (User, error) {
	query := fmt.Sprintf("SELECT %s FROM users WHERE login=$1 OR email=$1", userCol)
	row := s.db.QueryRow(query, str)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Login, &u.Password, &u.Age, &u.Gender, &u.FirstName, &u.LastName)
	if err != nil {
		return User{}, common.NotFoundError(nil, "cannot find user with this login")
	}
	return u, nil
}

func (s *Service) createSession(userID, sessionID string) error {
	query := fmt.Sprintf("INSERT INTO sessions (%s) VALUES ($1, $2, $3)", sessionCol)
	t := time.Now().Add(24 * time.Hour)
	_, err := s.db.Exec(query, sessionID, userID, t)

	if err != nil {
		// update session_key if exist
		if err.Error() == "UNIQUE constraint failed: sessions.user_id" {
			query := fmt.Sprintf("UPDATE sessions SET session_key=$1 WHERE user_id=$2")
			t := time.Now().Add(24 * time.Hour)
			_, err := s.db.Exec(query, sessionID, userID, t)
			if err != nil {
				return common.SystemError(err)
			}
			return nil
		}
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

func (s *Service) LogOut(ID string) error {
	query := fmt.Sprintf("DELETE FROM sessions WHERE user_id=$1")
	_, err := s.db.Exec(query, ID)
	if err != nil {
		return common.InvalidArgumentError(err, "no current session")
	}
	return nil
}
