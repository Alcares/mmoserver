package transport

import (
	"strings"
	"testing"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func TestValidateUsername(t *testing.T) {
	const ok = pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
	tests := []struct {
		name string
		in   string
		want pb.AccountCreateRejection
	}{
		{name: "empty", in: "", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_SHORT},
		{name: "below minimum", in: "A", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_SHORT},
		{name: "at minimum", in: "Al", want: ok},
		{name: "at maximum", in: strings.Repeat("a", usernameMaxLength), want: ok},
		{name: "over maximum", in: strings.Repeat("a", usernameMaxLength+1), want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TOO_LONG},
		{name: "letters digits and a space", in: "Jo Jo 42", want: ok},
		{name: "non-ascii letters", in: "Żaneta", want: ok},
		{name: "leading space", in: " Bob", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_SURROUNDING_WHITESPACE},
		{name: "trailing space", in: "Bob ", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_SURROUNDING_WHITESPACE},
		{name: "punctuation", in: "bob_1", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_INVALID_CHARACTERS},
		{name: "control character", in: "Al\x00ice", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_INVALID_CHARACTERS},
		{name: "zero-width space", in: "Bo​b", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_INVALID_CHARACTERS},
		{name: "bidi override", in: "Bob‮evil", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_INVALID_CHARACTERS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateUsername(tt.in); got != tt.want {
				t.Fatalf("validateUsername(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	const ok = pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
	tests := []struct {
		name string
		in   string
		want pb.AccountCreateRejection
	}{
		{name: "too short", in: "a1!b2@", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_SHORT},
		{name: "too long", in: strings.Repeat("a1!", 25), want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_LONG},
		{name: "on the common list", in: "password1!", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_COMMON},
		{name: "common list ignores case", in: "PASSWORD1!", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_TOO_COMMON},
		{name: "no number", in: "tradingpost!", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_MISSING_NUMBERS},
		{name: "no special character", in: "tradingpost7", want: pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_PASSWORD_MISSING_SPECIAL_CHARACTERS},
		{name: "acceptable", in: "tradingpost7!", want: ok},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatePassword(tt.in); got != tt.want {
				t.Fatalf("validatePassword(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
