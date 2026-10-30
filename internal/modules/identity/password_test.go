package identity

import (
	"strings"
	"testing"
)

func TestPasswordHash(test *testing.T) {
	password := "a long private password with spaces "
	first, err := HashPassword(password)
	if err != nil {
		test.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		test.Fatal(err)
	}
	if first == second || !CheckPassword(first, password) || CheckPassword(first, strings.TrimSpace(password)) || CheckPassword(first, "wrong") {
		test.Fatal("salt, exact password or verification policy violated")
	}
	for _, encoded := range []string{"", "plain text", strings.Replace(first, "m=19456", "m=999999999", 1), first + "$extra"} {
		if CheckPassword(encoded, password) {
			test.Fatal("accepted malformed hash")
		}
	}
	for _, password := range []string{"short", strings.Repeat("x", 257)} {
		if _, err := HashPassword(password); err == nil {
			test.Fatal("accepted invalid length")
		}
	}
}

func TestNormalizeEmail(test *testing.T) {
	canonical, err := NormalizeEmail(" Developer@Example.test ")
	if err != nil || canonical != "developer@example.test" {
		test.Fatalf("%s %v", canonical, err)
	}
	for _, input := range []string{"", "not-an-email", "Display <hello@example.test>", "a@example.test\nb@example.test"} {
		if _, err := NormalizeEmail(input); err == nil {
			test.Fatalf("accepted %q", input)
		}
	}
}
