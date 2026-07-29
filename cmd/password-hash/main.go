package main

import (
	"fmt"
	"os"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
)

func main() {
	password := os.Getenv("VF_BOOTSTRAP_PASSWORD")
	if password == "" {
		fmt.Fprintln(os.Stderr, "VF_BOOTSTRAP_PASSWORD is required")
		os.Exit(1)
	}
	hash, err := auth.DefaultPasswordHasher().Hash(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
