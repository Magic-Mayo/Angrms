package args

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"gitlab.sweetwater.com/mike_mayo/slackbot/slices"
)

var err = godotenv.Load(".env")
var api = slack.New(os.Getenv("OAUTH_TOKEN"), slack.OptionDebug(true))
var nameSeparator = os.Getenv("NAME_SEPARATOR")

type Leaderboard struct {
	User string    `json:"user,omitempty"`
	Date time.Time `json:"date,omitempty"`
}

type Game struct {
	User        string        `json:"user"`
	Id          int           `json:"id"`
	Active      bool          `json:"active"`
	Date        time.Time     `json:"date"`
	Words       []string      `json:"words"`
	Leaderboard []Leaderboard `json:"leaderboard,omitempty"`
	Letters     string        `json:"letters"`
	Private     bool          `json:"private,omitempty"`
	Expiration  string        `json:"expiration,omitempty"`
}

type GameOption struct {
	User        string
	Letters     string
	UsersSolved int
}

func CheckArgs(res http.ResponseWriter, command slack.SlashCommand) {
	args := strings.Fields(command.Text)

	if len(args) == 0 {
		mainMenu(res, command)
	} else {
		switch args[0] {
		case "create":
			createGame(command.UserName, command.TriggerID)
		case "play":
			findGame(res, command)
		case "stats":
			StatsInitView(res, command.TriggerID, false)
		default:
			res.Write([]byte("Only the following commands are available:\n`/angrm create`\n`/angrm play`\n`/angrm stats`"))
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
		if game.Active && !game.Private {
			if len(game.Leaderboard) == 0 {
				spliced = append(spliced, game)
				continue
			}
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

func getOwnPrivateGames(games []Game, user string) []Game {
	var spliced []Game
	for _, game := range games {
		if game.Private && game.User == user {
			spliced = append(spliced, game)
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
	user := strings.Split(userName, nameSeparator)
	titleCase := strings.ToUpper(strings.Split(user[0], "")[0])
	firstname := titleCase + strings.Join(strings.Split(user[0], "")[1:], "")
	lastTitleCase := strings.ToUpper(strings.Split(user[1], "")[0])
	lastname := lastTitleCase + strings.Join(strings.Split(user[1], "")[1:], "")
	fullname := firstname + " " + lastname

	return firstname, lastname, fullname
}

func createGameModal(user string) slack.ModalViewRequest {
	firstname, _, _ := getUser(user)

	message := "Hey *" + firstname + "*, go ahead and choose some letters to get a game started!"
	var modal slack.ModalViewRequest

	modal.CallbackID = "create"
	modal.Type = slack.ViewType("modal")

	modal.Close = slack.NewTextBlockObject("plain_text", "Cancel", false, false)
	modal.Title = slack.NewTextBlockObject("plain_text", "Angrms", false, false)
	modal.Submit = slack.NewTextBlockObject("plain_text", "Create Game", false, false)

	header := slack.NewTextBlockObject("mrkdwn", message, false, false)
	headerSection := slack.NewSectionBlock(header, nil, nil)
	divider := slack.NewDividerBlock()

	inputLabel := slack.NewTextBlockObject("plain_text", "Letters to create game with", false, false)
	inputPlaceholder := slack.NewTextBlockObject("plain_text", "rstlne", false, false)
	inputBlock := slack.NewPlainTextInputBlockElement(inputPlaceholder, "letters")
	input := slack.NewInputBlock("letters", inputLabel, nil, inputBlock)

	expirationMessage := "How long would you like this game to be active? Hint: use a number followed by a time unit. If you don't want to set a time limit, leave this field blank"
	expirationLabel := slack.NewTextBlockObject("plain_text", expirationMessage, false, false)
	expirationHint := slack.NewTextBlockObject("plain_text", "Expiration untis: m = minutes, h = hours, d = days", false, false)
	expirationPlaceholder := slack.NewTextBlockObject("plain_text", "3d", false, false)
	expirationInput := slack.NewPlainTextInputBlockElement(expirationPlaceholder, "expiration")
	expirationBlock := slack.NewInputBlock("expiration", expirationLabel, expirationHint, expirationInput)
	expirationBlock.Optional = true

	privateBlock := slack.NewTextBlockObject("plain_text", "Select to make this game playable by only you", false, false)
	privateOption := slack.NewOptionBlockObject("true", privateBlock, nil)
	privateCheckBox := slack.NewCheckboxGroupsBlockElement("private", privateOption)
	privateLabel := slack.NewTextBlockObject("plain_text", "Private game?", false, false)
	privateInput := slack.NewInputBlock("private", privateLabel, nil, privateCheckBox)
	privateInput.Optional = true

	modal.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			divider,
			expirationBlock,
			input,
			privateInput,
		},
	}

	return modal
}

func createGame(user string, triggerId string) {
	modal := createGameModal(user)

	apiRes, err := api.OpenView(triggerId, modal)

	if err != nil {
		fmt.Printf("%+v", apiRes)
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

func getExpiry(chosenExpiry string, creation time.Time) bool {
	length := len(chosenExpiry)
	unit := strings.ToLower(chosenExpiry[length-2 : length-1])
	amount, _ := strconv.Atoi(chosenExpiry[:length-2])
	timeSince := time.Since(creation)
	expired := false

	switch unit {
	case "m":
		if time.Duration.Minutes(timeSince) > float64(amount) {
			expired = true
		}
	case "h":
		if time.Duration.Hours(timeSince) > float64(amount) {
			expired = true
		}
	case "d":
		if time.Duration.Hours(timeSince) > float64(amount*24) {
			expired = true
		}
	}

	return expired
}

func SaveNewGame(payload slack.InteractionCallback, res http.ResponseWriter) {
	letters := payload.View.State.Values["letters"]["letters"].Value
	expiration := payload.View.State.Values["expiration"]["expiration"].Value
	options := payload.View.State.Values["private"]["private"].SelectedOptions
	private := false

	if len(options) > 0 {
		private = true
	}

	letters = removeDuplicates(letters)
	user := string(payload.User.Name)
	words := slices.FindWordsWithLetters(letters)

	view := updateModal(payload)

	if len(words) == 0 {
		messageMap := make(map[string]string)
		messageMap["letters"] = "No words found with letters '" + letters + "'!  Try another combination!"
		errors := slack.NewErrorsViewSubmissionResponse(messageMap)

		jsonString, _ := json.Marshal(errors)
		res.Header().Add("Content-Type", "application/json")
		res.Write(jsonString)
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

		viewRes := slack.NewUpdateViewSubmissionResponse(&view)

		jsonString, _ := json.Marshal(viewRes)

		res.Header().Add("Content-Type", "application/json")
		res.Write(jsonString)

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
		game.Private = private
		game.Expiration = expiration

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
		game := games[i]
		_, _, fullname := getUser(game.User)
		letters := game.Letters
		solved := len(game.Leaderboard)

		message := strings.ToTitle(letters) + " - " + strconv.Itoa(solved) + " users solved"
		optionText := slack.NewTextBlockObject("mrkdwn", message, false, false)

		descriptionText := fullname + " - " + strconv.Itoa(len(game.Words)) + " words"
		description := slack.NewTextBlockObject("plain_text", descriptionText, false, false)

		optionBlock := slack.NewOptionBlockObject(strconv.Itoa(game.Id), optionText, description)
		gameSelect := slack.NewRadioButtonsBlockElement(strconv.Itoa(game.Id), optionBlock)
		options = append(options, gameSelect)
	}

	actionBlock := slack.NewActionBlock("game", options...)
	return *actionBlock
}

func gameInput(gameId string) *slack.InputBlock {
	inputLabel := slack.NewTextBlockObject("plain_text", "Make a guess!", false, false)
	inputHint := slack.NewTextBlockObject("plain_text", "Game ID "+gameId, false, false)
	inputBlock := slack.NewPlainTextInputBlockElement(nil, "letters")
	input := slack.NewInputBlock("guess", inputLabel, inputHint, inputBlock)
	input.DispatchAction = true

	return input
}

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

	games := getGames(res)
	id, _ := strconv.Atoi(gameId)
	letters := "*" + strings.ToUpper(games[id].Letters) + "*"
	letterBlock := slack.NewTextBlockObject("mrkdwn", letters, false, false)
	letterSection := slack.NewSectionBlock(letterBlock, nil, nil)

	input := gameInput(gameId)

	view.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			letterSection,
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
	guess := strings.ToLower(req.View.State.Values["guess"]["letters"].Value)
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

	incorrectGuess := true

	for _, word := range game.Words {
		if guess == word {
			wordsFound = append(wordsFound, word)
			incorrectGuess = false
			break
		}
	}

	if incorrectGuess {
		errorMap := make(map[string]string)
		errorMap["guess"] = "'" + guess + "' is incorrect!"
		errors := slack.NewErrorsViewSubmissionResponse(errorMap)

		jsonString, _ := json.Marshal(errors)

		res.Header().Add("Content-Type", "application/json")
		res.Write(jsonString)
	} else if len(wordsFound) == len(game.Words) {
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
			res.Write([]byte("Could not marshal json"))
		}

		os.WriteFile("games.json", jsonString, os.ModePerm)
	} else if len(wordsFound) > 0 {
		view.PrivateMetadata = strings.Join(wordsFound, ",")
		view.PrivateMetadata = meta[0] + "," + view.PrivateMetadata
		view.CallbackID = "play"
		view.Blocks = req.View.Blocks

		totalWords := strconv.Itoa(len(game.Words) - len(wordsFound))

		headerText := totalWords + " words left!"
		header := slack.NewTextBlockObject("plain_text", headerText, false, false)
		headerSection := slack.NewSectionBlock(header, nil, nil)

		view.Blocks.BlockSet[1] = headerSection

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
	}
}

func findGameModal(res http.ResponseWriter, user string, private bool) (slack.ModalViewRequest, []Game) {
	var games []Game
	games = getGames(res)
	if private {
		games = getOwnPrivateGames(games, user)
	} else {
		games = getActiveGames(games, user)
	}
	spew.Dump(games)
	firstname, _, _ := getUser(user)

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

	return view, games
}

func findGame(res http.ResponseWriter, command slack.SlashCommand) {
	params := strings.Fields(command.Text)
	var private bool
	if len(params) < 2 {
		private = false
	} else if params[1] == "private" {
		private = true
	}
	view, games := findGameModal(res, command.UserName, private)

	if len(games) == 0 {
		message := "Could not find any games :cry:"
		messageBlock := slack.NewTextBlockObject("plain_text", message, false, false)
		messageSection := slack.NewSectionBlock(messageBlock, nil, nil)
		view.Blocks.BlockSet = []slack.Block{
			messageSection,
		}
	}

	apiRes, err := api.OpenView(command.TriggerID, view)

	if err != nil {
		fmt.Printf("%+v", apiRes)
		return
	}
}

func mainMenu(res http.ResponseWriter, command slack.SlashCommand) {
	firstname, _, _ := getUser(command.UserName)
	var view slack.ModalViewRequest
	view.Type = slack.ViewType("modal")
	view.CallbackID = "main"
	view.Title = slack.NewTextBlockObject("plain_text", "Angrms Main Menu", false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Close", false, false)
	view.ClearOnClose = true

	welcome := ":wave: " + firstname + "!  If you haven't played yet, here are some pointers to get you started when creating a game. \nGo ahead and create a game or find one to play :point_down::point_down::point_down:"
	messageBlock := slack.NewTextBlockObject("plain_text", welcome, false, false)
	messageSection := slack.NewSectionBlock(messageBlock, nil, nil)

	createMessage := "1. Provide some letters to create the game with.\n2. Duplicate letters aren't necessary.  The game will use the letters given multiple times if it can.\n3. You will *_only_* be shown the amount of words that are created but *_not_* the words themselves."
	createBlock := slack.NewTextBlockObject("mrkdwn", createMessage, false, false)
	createSection := slack.NewSectionBlock(createBlock, nil, nil)
	playMessage := "1. You will guess one word at a time\n2. If a word is correct it will show up at the bottom\n3. When you find all the words in a game you will be added to that game's leaderboard.\n4. Have fun! :confetti_ball:"
	playBlock := slack.NewTextBlockObject("plain_text", playMessage, false, false)
	playSection := slack.NewSectionBlock(playBlock, nil, nil)

	playButtonMessage := slack.NewTextBlockObject("plain_text", "Choose a game to play", false, false)
	playButtonText := slack.NewTextBlockObject("plain_text", "Play", false, false)
	playButton := slack.NewButtonBlockElement("play", "play", playButtonText)
	playButtonAccessory := slack.NewAccessory(playButton)
	playButtonSection := slack.NewSectionBlock(playButtonMessage, nil, playButtonAccessory)
	playButtonSection.BlockID = "play"

	playPrivateButtonMessage := slack.NewTextBlockObject("plain_text", "Play one of your private games", false, false)
	playPrivateButtonText := slack.NewTextBlockObject("plain_text", "Play Private", false, false)
	playPrivateButton := slack.NewButtonBlockElement("play-private", "play-private", playPrivateButtonText)
	playPrivateButtonAccessory := slack.NewAccessory(playPrivateButton)
	playPrivateButtonSection := slack.NewSectionBlock(playPrivateButtonMessage, nil, playPrivateButtonAccessory)
	playPrivateButtonSection.BlockID = "play-private"

	createButtonMessage := slack.NewTextBlockObject("plain_text", "Create a new game", false, false)
	createButtonText := slack.NewTextBlockObject("plain_text", "Create", false, false)
	createButton := slack.NewButtonBlockElement("create", "create", createButtonText)
	createButtonAccessory := slack.NewAccessory(createButton)
	createButtonSection := slack.NewSectionBlock(createButtonMessage, nil, createButtonAccessory)
	createButtonSection.BlockID = "create"

	statsMessage := slack.NewTextBlockObject("plain_text", "Check out the leaderboards", false, false)
	statsButtonText := slack.NewTextBlockObject("plain_text", "Stats", false, false)
	statsButton := slack.NewButtonBlockElement("stats", "stats", statsButtonText)
	statsButtonAccessory := slack.NewAccessory(statsButton)
	statsSection := slack.NewSectionBlock(statsMessage, nil, statsButtonAccessory)

	view.Blocks.BlockSet = []slack.Block{
		messageSection,
		createSection,
		createButtonSection,
		playSection,
		playButtonSection,
		playPrivateButtonSection,
		statsSection,
	}

	apiRes, err := api.OpenView(command.TriggerID, view)

	if err != nil {
		fmt.Printf("%+v", apiRes)
		return
	}
}

func ParseMenu(req slack.InteractionCallback, res http.ResponseWriter) {
	selectedOption := req.ActionCallback.BlockActions[0].ActionID

	var view slack.ModalViewRequest
	switch selectedOption {
	case "create":
		view = createGameModal(req.User.Name)
	case "play":
		modal, games := findGameModal(res, req.User.Name, false)
		view = modal

		if len(games) == 0 {
			viewResponse := slack.NewTextBlockObject("plain_text", "Could not find any games :cry:", false, false)
			viewSection := slack.NewSectionBlock(viewResponse, nil, nil)
			view.Blocks.BlockSet = []slack.Block{
				viewSection,
			}
		}
	case "play-private":
		modal, games := findGameModal(res, req.User.Name, true)
		view = modal

		if len(games) == 0 {
			viewResponse := slack.NewTextBlockObject("plain_text", "Could not find any games :cry:", false, false)
			viewSection := slack.NewSectionBlock(viewResponse, nil, nil)
			view.Blocks.BlockSet = []slack.Block{
				viewSection,
			}
		}
	case "stats":
		StatsInitView(res, req.TriggerID, true)
		return
	}

	apiRes, err := api.UpdateView(view, "", req.Hash, req.View.ID)

	if err != nil {
		fmt.Printf("%+v", apiRes)
		return
	}
}

func StatsInitView(res http.ResponseWriter, triggerID string, push bool) {
	games := addGameOptions(0, getGames(res))
	var view slack.ModalViewRequest
	view.Type = slack.ViewType("modal")
	view.CallbackID = "gamestats"

	view.Title = slack.NewTextBlockObject("plain_text", "Angrms Stats", false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Close", false, false)
	view.ClearOnClose = true

	header := "Choose a game to see all the users who solved!"
	headerBlock := slack.NewTextBlockObject("plain_text", header, false, false)
	headerSection := slack.NewSectionBlock(headerBlock, nil, nil)

	view.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			slack.NewDividerBlock(),
			games,
		},
	}

	var err error
	var apiRes *slack.ViewResponse
	if push {
		apiRes, err = api.PushView(triggerID, view)
	} else {
		apiRes, err = api.OpenView(triggerID, view)
	}

	if err != nil {
		fmt.Printf("%+v", apiRes)
		return
	}
}

func ShowStats(req slack.InteractionCallback, res http.ResponseWriter) {
	gameID, _ := strconv.Atoi(req.ActionCallback.BlockActions[0].SelectedOption.Value)
	game := getGames(res)[gameID]
	solvedLayout := "_2 Jan 2006 3:04:05 PM"
	layout := "_2 Jan 2006 3:04 PM"

	var view slack.ModalViewRequest
	view.Type = slack.ViewType("modal")
	view.Title = slack.NewTextBlockObject("plain_text", "Angrms Stats", false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Close", false, false)

	header := game.User + "'s Game - " + game.Date.Local().Format(layout)
	headerBlock := slack.NewTextBlockObject("plain_text", header, false, false)
	headerSection := slack.NewSectionBlock(headerBlock, nil, nil)

	var board []slack.Block
	board = append(board, headerSection)
	for i, solved := range game.Leaderboard {
		position := strconv.Itoa(i + 1)
		_, _, user := getUser(solved.User)
		date := solved.Date.Local().Format(solvedLayout)

		row := slack.NewTextBlockObject("mrkdwn", "*"+position+")*  _"+user+"_ - "+date, false, false)
		rowSection := slack.NewSectionBlock(row, nil, nil)
		board = append(board, rowSection)
	}

	view.Blocks.BlockSet = board

	apiRes, err := api.UpdateView(view, "", req.Hash, req.View.ID)

	if err != nil {
		fmt.Printf("%+v", apiRes)
		return
	}
}
