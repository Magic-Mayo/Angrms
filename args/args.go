package args

import (
	"encoding/json"
	"fmt"
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
var nameSeparator = os.Getenv("NAME_SEPARATOR")

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

type GameOption struct {
	User        string
	Letters     string
	UsersSolved int
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
			findGame(res, command)
		case "find":
			// findGame(res)
		default:
			res.Write([]byte("Please use one of the following arguments to get started: create, play, or find"))
		}
	}
}

func getGames(res http.ResponseWriter) []Game {
	file, err := os.ReadFile("games.json")

	if err != nil {
		res.Write([]byte("Could not open games file"))
		return nil
	}

	var games []Game
	json.Unmarshal(file, &games)

	return games
}

func getActiveGames(games []Game, user string) []Game {
	var spliced []Game
	for _, game := range games {
		if game.Active {
			for _, board := range game.Leaderboard {
				if strings.Contains(board.User, user) {
					break
				}
				spliced = append(spliced, game)
			}
		}
	}

	return spliced
}

func updateModal(payload slack.InteractionCallback) slack.ModalViewRequest {
	var view slack.ModalViewRequest
	view.CallbackID = payload.CallbackID
	view.Type = payload.View.Type
	view.Title = payload.View.Title
	view.Close = payload.View.Close
	view.Submit = payload.View.Submit

	return view
}

func getUser(userName string) (string, string, string) {
	// TODO: change '.' to '_' after testing...or make into variable
	user := strings.Split(userName, nameSeparator)
	titleCase := strings.ToUpper(strings.Split(user[0], "")[0])
	firstname := titleCase + strings.Join(strings.Split(user[0], "")[1:], "")
	lastTitleCase := strings.ToUpper(strings.Split(user[1], "")[0])
	lastname := lastTitleCase + strings.Join(strings.Split(user[1], "")[1:], "")
	fullname := firstname + " " + lastname

	return firstname, lastname, fullname
}

func createGame(res http.ResponseWriter, command slack.SlashCommand) {
	firstname, _, _ := getUser(command.UserName)
	welcome := ":wave: " + firstname + "!  If you haven't played yet, here are some pointers to get you started.\n1. Provide some letters to create the game with.\n2. Duplicate letters aren't necessary.  The game will use the letters given multiple times if it can.\n3. You will *_only_* be shown the amount of words that are created but *_not_* the words themselves.\n4. Have fun!"
	var modal slack.ModalViewRequest

	modal.CallbackID = "create"
	modal.Type = slack.ViewType("modal")

	modal.Close = slack.NewTextBlockObject("plain_text", "Cancel", false, false)
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

func removeDuplicates(letters string) string {
	stringSet := make(map[string]bool, 0)
	letterSet := ""

	for _, letter := range strings.Split(letters, "") {
		if !stringSet[letter] {
			stringSet[letter] = true
			letterSet += letter
		}
	}

	return letterSet
}

func SaveNewGame(payload slack.InteractionCallback, res http.ResponseWriter) {
	letters := payload.View.State.Values["letters"]["letters"].Value
	letters = removeDuplicates(letters)
	user := string(payload.User.Name)
	words := slices.FindWordsWithLetters(letters)

	view := updateModal(payload)

	if len(words) == 0 {
		message := "No words found with letters '" + letters + "'!  Try another combination!"
		text := slack.NewTextBlockObject("mrkdwn", message, false, false)
		textBlock := slack.NewSectionBlock(text, nil, nil)

		view.Blocks.BlockSet = append(view.Blocks.BlockSet, textBlock)

		_, err := api.UpdateView(view, payload.View.ExternalID, payload.Hash, payload.View.ID)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		message := "You created a game that has " + strconv.Itoa(len(words)) + " words to find! 🚀🚀🚀"
		textBlock := slack.NewTextBlockObject("plain_text", message, false, false)
		section := slack.NewSectionBlock(textBlock, nil, nil)

		blocks := slack.Blocks{
			BlockSet: []slack.Block{
				section,
			},
		}

		view.Blocks = blocks
		view.Submit = nil
		view.ClearOnClose = true
		view.Close.Text = "Close"

		_, err := api.PushView(payload.TriggerID, view)

		if err != nil {
			fmt.Print(err)
			return
		}

		games := getGames(res)
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

func addGameOptions(offset int, games []Game) slack.ActionBlock {
	var options []slack.BlockElement
	for i := offset; i < len(games); i++ {
		_, _, fullname := getUser(games[i].User)
		letters := games[i].Letters
		solved := len(games[i].Leaderboard)

		message := strings.ToTitle(letters) + " - " + strconv.Itoa(solved) + " users solved"
		optionText := slack.NewTextBlockObject("mrkdwn", message, false, false)

		descriptionText := fullname + " - " + strconv.Itoa(len(games[i].Words)) + " words"
		description := slack.NewTextBlockObject("plain_text", descriptionText, false, false)

		optionBlock := slack.NewOptionBlockObject(strconv.Itoa(i), optionText, description)
		gameSelect := slack.NewRadioButtonsBlockElement(strconv.Itoa(i), optionBlock)
		options = append(options, *gameSelect)
	}

	actionBlock := slack.NewActionBlock("game", options...)
	return *actionBlock
}

func playGame(res http.ResponseWriter, command slack.SlashCommand) {
	// games := getGames(res)

}

// getGuesses(guess ...string) []string {

// }

func StartGame(req slack.InteractionCallback, res http.ResponseWriter) {
	var view slack.ModalViewRequest
	view.CallbackID = "play"

	selectedGame := req.ActionCallback.BlockActions[0].SelectedOption
	gameId := selectedGame.Value
	splitDescriptiom := strings.Split(selectedGame.Description.Text, " - ")
	creator := splitDescriptiom[0]
	totalWords := strings.Split(splitDescriptiom[1], " words")[0]
	view.PrivateMetadata = gameId

	if len(creator) > 18 {
		creator = strings.Split(creator, " ")[0]
	}

	titleMessage := creator + "'s game"
	view.Type = slack.ViewType("modal")
	view.Title = slack.NewTextBlockObject("plain_text", titleMessage, false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Cancel", false, false)

	headerText := totalWords + " words left!"
	header := slack.NewTextBlockObject("plain_text", headerText, false, false)
	headerSection := slack.NewSectionBlock(header, nil, nil)

	inputLabel := slack.NewTextBlockObject("plain_text", "Make a guess!", false, false)
	inputHint := slack.NewTextBlockObject("plain_text", "Game ID "+gameId, false, false)
	inputBlock := slack.NewPlainTextInputBlockElement(nil, "letters")
	input := slack.NewInputBlock("letters", inputLabel, inputHint, inputBlock)
	input.DispatchAction = true

	view.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			input,
		},
	}

	apiRes, err := api.PushView(req.TriggerID, view)

	if err != nil {
		fmt.Println(err, apiRes)
		return
	}
}

func PlayGame(req slack.InteractionCallback, res http.ResponseWriter) {
	games := getGames(res)
	guess := strings.ToLower(req.View.State.Values["letters"]["letters"].Value)

	view := updateModal(req)

	if games == nil {
		return
	}

	meta := strings.Split(req.View.PrivateMetadata, ",")
	gameId, _ := strconv.Atoi(meta[0])

	game := games[gameId]
	user := req.User.Name

	var wordsFound []string
	if len(meta) > 1 {
		wordsFound = meta[1:]
	}

	for _, word := range game.Words {
		if word == guess {
			wordsFound = append(wordsFound, word)
		}
	}
	fmt.Printf("%+v", wordsFound)

	if len(wordsFound) == len(game.Words) {
		_, _, creator := getUser(game.User)
		var view slack.ModalViewRequest
		view.Title = slack.NewTextBlockObject("plain_text", "Solved!! 🎉🎉🎉", false, false)
		view.ClearOnClose = true
		view.Close = slack.NewTextBlockObject("plain_text", "Close", false, false)
		view.Type = slack.ViewType("modal")

		message := "Congrats, you found all the words in " + creator + "'s game! You will be added to this game's leaderboard :wink:.  Use the `/angrms stats` to check it out!"
		messageBlock := slack.NewTextBlockObject("mrkdwn", message, false, false)
		sectionBlock := slack.NewSectionBlock(messageBlock, nil, nil)

		view.Blocks = slack.Blocks{
			BlockSet: []slack.Block{
				sectionBlock,
			},
		}

		apiRes, err := api.PushView(req.TriggerID, view)

		if err != nil {
			fmt.Printf("%+v", apiRes)
		}

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
		view.PrivateMetadata = strings.Join(wordsFound, ",")
		view.PrivateMetadata = meta[0] + "," + view.PrivateMetadata
		view.CallbackID = "play"

		view.Blocks = req.View.Blocks

		if len(view.Blocks.BlockSet) == 2 {
			divider := slack.NewDividerBlock()
			message := slack.NewTextBlockObject("plain_text", "You found: ", false, false)
			messageBlock := slack.NewSectionBlock(message, nil, nil)

			view.Blocks.BlockSet = append(view.Blocks.BlockSet, []slack.Block{
				divider,
				messageBlock,
			}...)
		}

		newWord := slack.NewTextBlockObject("plain_text", wordsFound[len(wordsFound)-1], false, false)
		newWordSection := slack.NewSectionBlock(newWord, nil, nil)
		view.Blocks.BlockSet = append(view.Blocks.BlockSet, newWordSection)

		apiRes, err := api.UpdateView(view, req.View.ExternalID, req.Hash, req.View.ID)

		if err != nil {
			fmt.Printf("%+v", apiRes)
			return
		}
	} else {
		res.Write([]byte("You didn't find any valid words!"))
	}
}

func findGame(res http.ResponseWriter, command slack.SlashCommand) {
	games := getActiveGames(getGames(res), command.UserName)

	firstname, _, _ := getUser(command.UserName)

	var view slack.ModalViewRequest

	view.CallbackID = "find"
	view.Type = slack.ViewType("modal")
	view.Title = slack.NewTextBlockObject("plain_text", "Choose a game", false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Cancel", false, false)

	message := "Hey there " + firstname + "!  Choose a game from the options below to start playing!"

	header := slack.NewTextBlockObject("mrkdwn", message, false, false)
	headerSection := slack.NewSectionBlock(header, nil, nil)
	divider := slack.NewDividerBlock()

	gameOptions := addGameOptions(0, games)

	view.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			divider,
			gameOptions,
		},
	}

	if len(games) == 0 {

	}

	apiRes, err := api.OpenView(command.TriggerID, view)
	if err != nil {
		fmt.Print(err, apiRes)
		return
	}
}

func MainMenu(res http.ResponseWriter) {

}
