package user

import (
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Login      string `json:"login"`
	Password   string `json:"password,omitempty"`
	RepeatPWD  string `json:"repeat_pwd,omitempty"`
	Age        uint   `json:"age"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Gender     Gender `json:"gender"`
	GenderText string `json:"gender_text"`
}

type Gender uint8

func (g Gender) String() string {
	switch g {
	case Male:
		return "male"
	case Female:
		return "female"
	case Other:
		return "other"
	default:
		return ""
	}
}

const (
	Other Gender = iota
	Male
	Female
)

func (u *User) generateID() {
	u.ID = uuid.NewV4().String()
}

func (u *User) hashPassword() {
	hash, _ := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	u.Password = string(hash)
}

func (u *User) comparePassword(hash, pw string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	return err == nil
}

func (u *User) cleanUp() {
	u.Password = ""
	u.RepeatPWD = ""
	u.GenderText = u.Gender.String()
}
