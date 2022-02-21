package user

import (
	"forum/internal/common"
	"net/mail"
	"regexp"
)

func validateUser(u User) error {
	if err := validateLogin(u.Login); err != nil {
		return err
	}
	if err := validateEmail(u.Email); err != nil {
		return err
	}
	if err := validatePwd(u.Password, u.RepeatPWD); err != nil {
		return err
	}
	if err := validateName(u.FirstName, u.LastName); err != nil {
		return err
	}
	if err := validateAge(u.Age); err != nil {
		return err
	}
	return nil
}

var (
	minLenPwd = 8
	isCorrect = regexp.MustCompile(`[0-9a-zA-Z]{3,255}$`).MatchString
)

func validateLogin(login string) error {
	if login == "" {
		return common.InvalidArgumentError(nil, "login is empty")
	}
	if !isCorrect(login) {
		return common.InvalidArgumentError(nil, "login format is invalid")
	}
	return nil
}

func validateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return common.InvalidArgumentError(nil, "email format is invalid")
	}
	return nil
}

func validatePwd(pwd, repeatPWD string) error {
	if pwd == "" {
		return common.InvalidArgumentError(nil, "password cannot be empty")
	}
	if len(pwd) < minLenPwd {
		return common.InvalidArgumentError(nil, "password is too short")
	}
	if pwd != repeatPWD {
		return common.InvalidArgumentError(nil, "passwords do not match")
	}
	return nil
}

func validateName(first, last string) error {
	if first == "" {
		return common.InvalidArgumentError(nil, "first name is required")

	}
	if last == "" {
		return common.InvalidArgumentError(nil, "last name is required")
	}
	return nil
}

func validateAge(age uint) error {
	if age == 0 {
		return common.InvalidArgumentError(nil, "age is empty")
	}
	return nil
}
