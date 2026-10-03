package main

import (
	"fmt"
	"io"
	"os"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
)

func main() {
	password := []byte(os.Getenv("VF_BOOTSTRAP_PASSWORD"))
	if len(password) == 0 {
		var err error
		password, err = io.ReadAll(io.LimitReader(os.Stdin, 129))
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not read password from stdin")
			os.Exit(1)
		}
	}
	if len(password) < 8 || len(password) > 128 {
		fmt.Fprintln(os.Stderr, "expected an 8–128 byte password on stdin")
		os.Exit(1)
	}
	hash, err := auth.DefaultPasswordHasher().Hash(string(password))
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not hash password")
		os.Exit(1)
	}
	fmt.Println(hash)
}
