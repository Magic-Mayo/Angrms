package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"gitlab.sweetwater.com/mike_mayo/slackbot/slackHandler"
)

var err = godotenv.Load(".env")

func main() {
	http.HandleFunc("/", slackHandler.SlashCommandHandler)
	// http.HandleFunc("/interactive", slackHandler.InteractiveHandler)

	port := os.Getenv("PORT")
	fmt.Println("Just felt like running.... http://localhost" + port)
	defer fmt.Println("I think I'll go home now")

	log.Fatal(http.ListenAndServe(port, nil))
}
