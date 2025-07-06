package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Check if a password was provided as a command-line argument.
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <password>")
	}

	password := os.Args[1]

	// Generate the bcrypt hash. The second argument is the cost factor.
	// bcrypt.DefaultCost (10) is a good balance of security and speed.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Print the hash to the console. You can then copy and paste this.
	fmt.Println(string(hashedPassword))
}
