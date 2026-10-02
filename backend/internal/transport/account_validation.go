package transport

import (
	_ "embed"
	"strings"
	"unicode"
	"unicode/utf8"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

const (
	passwordMinLength       = 8
	passwordMaxLength       = 72
	passwordMinNumbers      = 1
	passwordMinSpecialChars = 1

	usernameMinLength = 2
	usernameMaxLength = 20
)

//go:embed common_passwords.txt
var commonPasswordsFile string

// commonPasswords holds the embedded list, lowercased, built once at startup
var commonPasswords = func() map[string]struct{} {
	m := make(map[string]struct{})
	for line := range strings.SplitSeq(commonPasswordsFile, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			m[strings.ToLower(p)] = struct{}{}
		}
	}
	return m
}()

func validatePassword(password string) pb.AccountCreateRejection {
	switch {
	case utf8.RuneCountInString(password) < passwordMinLength:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_SHORT
	case len(password) > passwordMaxLength: // bcrypt's limit is 72 bytes
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_LONG
	}

	lower := strings.ToLower(password)
	if _, common := commonPasswords[lower]; common {
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_COMMON
	}

	var numbers, specialChars int
	r := []rune(password)
	for _, c := range r {
		if unicode.IsNumber(c) {
			numbers++
		}
		if unicode.IsSymbol(c) || unicode.IsPunct(c) {
			specialChars++
		}
	}

	switch {
	case numbers < passwordMinNumbers:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_MISSING_NUMBERS
	case specialChars < passwordMinSpecialChars:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_MISSING_SPECIAL_CHARACTERS
	}

	return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}

func validateUsername(username string) pb.AccountCreateRejection {
	switch {
	case username != strings.TrimSpace(username):
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_SURROUNDING_WHITESPACE
	case utf8.RuneCountInString(username) < usernameMinLength:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_SHORT
	case utf8.RuneCountInString(username) > usernameMaxLength:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_LONG
	}

	r := []rune(username)
	for _, c := range r {
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) && c != ' ' {
			return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_INVALID_CHARACTERS
		}
	}

	return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
}
