package auth

import (
	"fmt"
	"strings"
)

// commonPasswords are passwords that appear at the top of the public leaked
// password lists and that are at least minPasswordLength characters long
// (shorter ones are already refused). Compared case-insensitively. This is a
// short list on purpose: it stops the guesses an attacker tries first, without
// sending anything about the user's password to a third party. It is not a
// breach check.
var commonPasswords = func() map[string]struct{} {
	list := []string{
		"password", "password1", "password12", "password123", "password1234", "passw0rd", "p@ssw0rd", "p@ssword",
		"pass1234", "pass12345", "passcode", "letmein1", "letmein123", "welcome1", "welcome12", "welcome123",
		"12345678", "123456789", "1234567890", "12345678910", "11111111", "111111111", "1111111111", "00000000",
		"000000000", "0000000000", "12341234", "123123123", "1234abcd", "abcd1234", "abc12345", "abcd12345",
		"qwertyui", "qwerty12", "qwerty123", "qwerty1234", "qwertyuiop", "qwerty123456", "asdfghjk", "asdfghjkl",
		"asdf1234", "zxcvbnm1", "zxcvbnm123", "1q2w3e4r", "1q2w3e4r5t", "1qaz2wsx", "1qaz2wsx3edc", "q1w2e3r4",
		"q1w2e3r4t5", "iloveyou", "iloveyou1", "iloveyou123", "iloveyou2", "ilovekids", "monkey123", "dragon123",
		"football1", "football123", "baseball1", "basketball", "superman1", "batman123", "princess1", "sunshine1",
		"sunshine123", "trustno1", "master123", "shadow123", "michael1", "jessica1", "charlie1", "jennifer1",
		"changeme", "changeme1", "changeme123", "admin123", "admin1234", "administrator", "root1234", "test1234",
		"testtest", "test12345", "testing123", "default123", "guest123", "login123", "user1234", "hello123",
		"hello1234", "helloworld", "india123", "india1234", "bharat123", "krishna123", "ganesh123", "shiva123",
		"sachin10", "sachin123", "rahul123", "rahul1234", "ramesh123", "suresh123", "mumbai123", "delhi123",
		"computer1", "internet1", "google123", "facebook1", "youtube1", "indifferent", "indifferent1", "indifferent123",
		"aaaaaaaa", "aaaa1111", "aaaaaa11", "abcdefgh", "abcdefghi", "abcdefghij", "987654321", "9876543210",
		"87654321", "09876543", "13579246", "135792468", "24681012", "147258369", "123456abc", "123456aa",
		"a1b2c3d4", "a1234567", "a12345678", "a123456789", "z1x2c3v4", "zaq12wsx", "qazwsxedc", "qazwsx123",
		"1234qwer", "qwer1234", "qwe12345", "qwe123456", "11223344", "112233445", "1122334455", "12121212",
		"69696969", "88888888", "666666666", "77777777", "99999999", "55555555", "22222222", "33333333", "44444444",
		"mypassword", "mypassword1", "mypass123", "secret123", "secret12", "supersecret", "letmeinnow", "access123",
		"starwars1", "pokemon1", "minecraft", "whatever1", "freedom1", "cheese123", "summer2020", "summer2021",
		"summer2022", "summer2023", "summer2024", "summer2025", "winter2024", "winter2025", "spring2025", "autumn2025",
		"password2024", "password2025", "password2026", "welcome2024", "welcome2025", "welcome2026", "india2025", "india2026",
	}
	set := make(map[string]struct{}, len(list))
	for _, p := range list {
		set[p] = struct{}{}
	}
	return set
}()

// checkPasswordStrength rejects the passwords an attacker tries first: a
// well-known password, one character repeated, or the person's own email name.
// It runs when a password is chosen (sign-up), never on login, so existing
// accounts are not locked out by it.
func checkPasswordStrength(email, password string) error {
	lower := strings.ToLower(password)
	if _, common := commonPasswords[lower]; common {
		return fmt.Errorf("password must be harder to guess: it is a commonly used password")
	}
	if strings.Trim(lower, string([]rune(lower)[:1])) == "" {
		return fmt.Errorf("password must be harder to guess: it is one repeated character")
	}
	local := strings.ToLower(strings.TrimSpace(email))
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}
	if len(local) >= 4 && strings.Contains(lower, local) {
		return fmt.Errorf("password must be harder to guess: it contains your email name")
	}
	return nil
}
