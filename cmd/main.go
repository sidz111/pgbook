package main

import (
	"fmt"

	"github.com/sidz111/pgbook/config"
)

func main() {
	fmt.Println("Welcome to PGBook - Your Ultimate PG Management Solution!")
	if err := config.ConnectDB(); err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}
	fmt.Println("Successfully connected to the database!")
}
