package args

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	handleResponse "gitlab.sweetwater.com/mike_mayo/slackbot/response"
	"gitlab.sweetwater.com/mike_mayo/slackbot/slices"
)

var err = godotenv.Load(".env")
var api = slack.New(os.Getenv("OAUTH_TOKEN"), slack.OptionDebug(true))

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

func CheckArgs(res http.ResponseWriter, command slack.SlashCommand) {
	args := strings.Fields(command.Text)
	if len(args) == 0 {
		res.Write([]byte("Please use one of the following arguments to get started: create, play, or find"))
	} else {
		switch args[0] {
		case "create":
			createGame(res, command)
		case "play":
			playGame(res, command)
		case "find":
			findGame(res)
		default:
			res.Write([]byte("Please use one of the following arguments to get started: create, play, or find"))
		}
	}
}

func createGame(res http.ResponseWriter, command slack.SlashCommand) {
	user := strings.Split(command.UserName, "_")[0]
	titleCase := strings.ToUpper(strings.Split(user, "")[0])
	firstname := titleCase + strings.Join(strings.Split(user, "")[1:], "")
	welcome := ":wave: " + firstname + "!  If you haven't played yet, here are some pointers to get you started.\n1. Provide some letters to create the game with.\n2. Duplicate letters aren't necessary.  The game will use the letters given multiple times if it can.\n3. You will *_only_* be shown the amount of words that are created but *_not_* the words themselves.\n4. Have fun!"
	var modal slack.ModalViewRequest

	modal.CallbackID = "create"
	modal.Type = slack.ViewType("modal")

	modal.Close = slack.NewTextBlockObject("plain_text", "Cancel", false, false)
	// modal.Submit = slack.NewTextBlockObject("plain_text", "Submit", false, false)
	modal.Title = slack.NewTextBlockObject("plain_text", "Angrms", false, false)

	header := slack.NewTextBlockObject("mrkdwn", welcome, false, false)
	headerSection := slack.NewSectionBlock(header, nil, nil)
	divider := slack.NewDividerBlock()

	inputLabel := slack.NewTextBlockObject("plain_text", "Letters to create game with", false, false)
	inputPlaceholder := slack.NewTextBlockObject("plain_text", "rstlne", false, false)
	inputBlock := slack.NewPlainTextInputBlockElement(inputPlaceholder, "letters")
	input := slack.NewInputBlock("letters", inputLabel, nil, inputBlock)
	input.DispatchAction = true

	modal.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			divider,
			input,
		},
	}

	res.Write([]byte(""))
	_, err := api.OpenView(command.TriggerID, modal)

	if err != nil {
		fmt.Println(err)
		return
	}
}

func SaveNewGame(payload slack.InteractionCallback, res http.ResponseWriter) {
	letters := payload.View.State.Values["letters"]["letters"].Value
	user := string(payload.User.Name)
	words := slices.FindWordsWithLetters(letters)

	var view slack.ModalViewRequest
	view.CallbackID = payload.CallbackID
	view.Type = payload.View.Type
	view.Title = payload.View.Title
	view.Close = payload.View.Close

	if len(words) == 0 {
		message := "No words found with letters '" + letters + "'!  Try another combination!"
		text := slack.NewTextBlockObject("mrkdwn", message, false, false)
		textBlock := slack.NewSectionBlock(text, nil, nil)

		blocks := payload.View.Blocks
		blocks.BlockSet = append(blocks.BlockSet, textBlock)

		view.Blocks = blocks

		_, err := api.UpdateView(view, payload.View.ExternalID, payload.Hash, payload.View.ID)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		message := "You created a game that has " + strconv.Itoa(len(words)) + " words to find! 🚀🚀🚀"
		textBlock := slack.NewTextBlockObject("plain_text", message, false, false)
		section := slack.NewSectionBlock(textBlock, nil, nil)

		view.ClearOnClose = true
		view.Close.Text = "Close"

		view.Blocks = slack.Blocks{
			BlockSet: []slack.Block{
				section,
			},
		}
		_, err := api.PushView(payload.TriggerID, view)

		if err != nil {
			fmt.Print(err)
			return
		}
		file, err := ioutil.ReadFile("games.json")

		if err != nil {
			fmt.Print(err)
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
			fmt.Print(err)
			return
		}

		os.WriteFile("games.json", jsonString, os.ModePerm)
	}
}

func playGame(res http.ResponseWriter, command slack.SlashCommand) {
	args := strings.Fields(command.Text)
	gameId, err := strconv.Atoi(args[1])
	guesses := args[2:]
	user := command.UserName

	if err != nil {
		handleResponse.HandleResponse(res, "Could not convert game id to int", 200)
	}
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
		fmt.Println(found, wordsFound)
		for word := range wordsFound {
			found = append(found, string(word))
		}
		handleResponse.HandleResponse(res, "You found: "+strings.Join(found, " and ")+"!", 200)
	} else {
		res.Write([]byte("You didn't find any valid words!"))
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
