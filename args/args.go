package args

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	handleResponse "gitlab.sweetwater.com/mike_mayo/slackbot/response"
	"gitlab.sweetwater.com/mike_mayo/slackbot/slices"
)

type Leaderboard struct {
	User string
	Date time.Time
}

type Game struct {
	User        string
	Id          int
	Active      bool
	Date        time.Time
	Words       []string
	Leaderboard []Leaderboard
	Letters     string
}

func CheckArgs(res http.ResponseWriter, args []string, user string) {
	switch args[0] {
	case "create":
		createGame(res, args[1], user)
	case "play":
		gameId, err := strconv.Atoi(args[1])

		if err != nil {
			handleResponse.HandleResponse(res, "Could not convert game id to int", 200)
		}

		playGame(res, gameId, args[2:], user)
	case "find":
		findGame(res)
	}
}

func createGame(res http.ResponseWriter, letters string, user string) {
	words := slices.FindWordsWithLetters(letters)

	if len(words) == 0 {
		handleResponse.HandleResponse(res, "No words found from '"+letters+"'", 200)
	} else {
		handleResponse.HandleResponse(res, words, 200)
		file, err := ioutil.ReadFile("games.json")

		if err != nil {
			handleResponse.HandleResponse(res, "Could not read games file", 200)
			return
		}

		var games []Game
		json.Unmarshal(file, &games)

		id := len(games)

		var game Game

		game.User = user
		game.Active = true
		game.Leaderboard = make([]Leaderboard, 0)
		game.Words = words
		game.Date = time.Now()
		game.Id = id
		game.Letters = letters

		games = append(games, game)

		jsonString, err := json.Marshal(games)

		if err != nil {
			handleResponse.HandleResponse(res, "Could not marshal games json", 200)
			return
		}

		os.WriteFile("games.json", jsonString, os.ModePerm)
	}
}

func playGame(res http.ResponseWriter, gameId int, guesses []string, user string) {
	file, err := os.ReadFile("games.json")

	if err != nil {
		handleResponse.HandleResponse(res, "Could not read games file", 200)
		return
	}

	var games []Game
	json.Unmarshal(file, &games)

	var game Game

	for _, cur := range games {
		if cur.Id == gameId {
			game = cur
			break
		}
	}
	wordsFound := make(map[string]bool)
	for _, word := range game.Words {
		for _, guess := range guesses {
			if word == guess {
				wordsFound[word] = true
			}
		}
	}

	if len(wordsFound) == len(game.Words) {
		handleResponse.HandleResponse(res, "You found all the words! 🎉🎉🎉", 200)

		game.Leaderboard = append(game.Leaderboard, Leaderboard{
			User: user,
			Date: time.Now(),
		})

		games[game.Id] = game

		jsonString, err := json.Marshal(games)

		if err != nil {
			handleResponse.HandleResponse(res, "Could not marshal json", 200)
		}

		os.WriteFile("games.json", jsonString, os.ModePerm)
	} else if len(wordsFound) > 0 {
		var found []string
		for word, _ := range wordsFound {
			found = append(found, string(word))
		}

		handleResponse.HandleResponse(res, "You found: "+strings.Join(found, " and ")+"!", 200)
	}
}

func findGame(res http.ResponseWriter) {
	file, err := ioutil.ReadFile("games.json")
	if err != nil {
		handleResponse.HandleResponse(res, "Could not read games file", 200)
		return
	}

	var games []Game

	json.Unmarshal(file, &games)

	var gamesFound [][]string

	for _, game := range games {
		if game.Active {
			var found []string
			found = append(found, strconv.Itoa(game.Id), game.User)
			gamesFound = append(gamesFound, found)
		}
	}

	handleResponse.HandleResponse(res, gamesFound, 200)
}
