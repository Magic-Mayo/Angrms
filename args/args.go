package args

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"gitlab.sweetwater.com/mike_mayo/slackbot/slices"
	"gitlab.sweetwater.com/mike_mayo/slackbot/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var err = godotenv.Load(".env")
var api = slack.New(os.Getenv("OAUTH_TOKEN"), slack.OptionDebug(true))
var nameSeparator = os.Getenv("NAME_SEPARATOR")
var client = util.MongoClient().Database("slack")

type Leaderboard struct {
	User string    `bson:"user,omitempty"`
	Date time.Time `bson:"date,omitempty"`
}

type Game struct {
	User        string             `bson:"user"`
	Id          primitive.ObjectID `bson:"_id,omitempty"`
	Active      bool               `bson:"active"`
	Date        time.Time          `bson:"date"`
	Words       []string           `bson:"words"`
	Leaderboard []Leaderboard      `bson:"leaderboard,omitempty"`
	Letters     string             `bson:"letters"`
	Private     bool               `bson:"private,omitempty"`
	Expiration  string             `bson:"expiration,omitempty"`
}

type GameOption struct {
	User        string
	Letters     string
	UsersSolved int
}

// type GamesStats struct {
// 	User   string `json:"user"`
// 	Amount int    `json:"amount"`
// }

// type Stats struct {
// 	Solved      []GameStats  `json:"solved,omitempty"`
// 	Created     []GamesStats `json:"created,omitempty"`
// 	UsersSolved []GamesStats `json:"usersSolved,omitempty"`
// }

func CheckArgs(res http.ResponseWriter, command slack.SlashCommand) {
	args := strings.Fields(command.Text)

	if len(args) == 0 {
		mainMenu(res, command)
	} else {
		switch args[0] {
		case "create":
			createGame(command.UserName, command.TriggerID)
		case "find":
			findGame(res, command)
		case "stats":
			StatsInitView(res, command.TriggerID, false)
		case "instructions", "rules", "tips":
			Instructions(command.TriggerID, res, false)
		default:
			res.Write([]byte("Only the following commands are available:\n`/angrms create`\n`/angrms play`\n`/angrms stats`\n`/angrms find`"))
		}
	}
}

func getGames(res http.ResponseWriter, filter bson.E) []Game {
	gamesColl := client.Collection("games")

	a := util.GetDocs(gamesColl, filter)

	var games []Game
	a.All(context.TODO(), &games)

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

	inputLabel := slack.NewTextBlockObject("plain_text", "Letters to create game with", false, false)
	inputPlaceholder := slack.NewTextBlockObject("plain_text", "rstlne", false, false)
	inputBlock := slack.NewPlainTextInputBlockElement(inputPlaceholder, "letters")
	input := slack.NewInputBlock("letters", inputLabel, nil, inputBlock)

	expirationMessage := "How long would you like this game to be active? Hint: use a number followed by a time unit. If you don't want to set a time limit, leave this field blank"
	expirationLabel := slack.NewTextBlockObject("plain_text", expirationMessage, false, false)
	expirationHint := slack.NewTextBlockObject("plain_text", "Supported units: m = minutes, h = hours, d = days", false, false)
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
			slack.NewDividerBlock(),
			input,
			expirationBlock,
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

		var game Game

		game.User = user
		game.Active = true
		game.Leaderboard = make([]Leaderboard, 0)
		game.Words = words
		game.Date = time.Now()
		game.Letters = letters
		game.Private = private
		game.Expiration = expiration

		insert, err := client.Collection("games").InsertOne(context.TODO(), game)

		if err != nil {
			fmt.Printf("%+v", insert)
			fmt.Printf("%+v", err)
		}
	}
}

func addGameOptions(offset int, games []Game) slack.ActionBlock {
	var options []slack.BlockElement
	for i := offset; i < len(games); i++ {
		game := games[i]
		gameID, _ := game.Id.MarshalText()
		_, _, fullname := getUser(game.User)
		letters := game.Letters
		solved := len(game.Leaderboard)

		message := strings.ToTitle(letters) + " - " + strconv.Itoa(solved) + " users solved"
		optionText := slack.NewTextBlockObject("mrkdwn", message, false, false)

		descriptionText := fullname + " - " + strconv.Itoa(len(game.Words)) + " words"
		description := slack.NewTextBlockObject("plain_text", descriptionText, false, false)

		optionBlock := slack.NewOptionBlockObject(string(gameID), optionText, description)
		gameSelect := slack.NewRadioButtonsBlockElement(string(gameID), optionBlock)
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
	gameId, err := primitive.ObjectIDFromHex(selectedGame.Value)

	if err != nil {
		fmt.Printf("%+v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	splitDescriptiom := strings.Split(selectedGame.Description.Text, " - ")
	creator := splitDescriptiom[0]
	totalWords := strings.Split(splitDescriptiom[1], " words")[0]
	view.PrivateMetadata = selectedGame.Value

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

	found := client.Collection("games").FindOne(context.TODO(), bson.D{{"_id", gameId}})

	var game Game
	found.Decode(&game)

	letters := "*" + strings.ToUpper(game.Letters) + "*"
	letterBlock := slack.NewTextBlockObject("mrkdwn", letters, false, false)
	letterSection := slack.NewSectionBlock(letterBlock, nil, nil)

	input := gameInput(selectedGame.Value)

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

func foundWordsSection(wordsFound []string) []slack.Block {
	var blocks []slack.Block
	message := slack.NewTextBlockObject("plain_text", "You found: ", false, false)
	messageBlock := slack.NewSectionBlock(message, nil, nil)

	blocks = append(blocks, []slack.Block{
		slack.NewDividerBlock(),
		messageBlock,
	}...)

	for _, word := range wordsFound {
		newWord := slack.NewTextBlockObject("plain_text", word, false, false)
		newWordSection := slack.NewSectionBlock(newWord, nil, nil)
		blocks = append(blocks, newWordSection)
	}

	return blocks
}

func alreadyGuessed(wordsFound []string, guess string) bool {
	for _, word := range wordsFound {
		if word == guess {
			return true
		}
	}

	return false
}

func PlayGame(req slack.InteractionCallback, res http.ResponseWriter) {
	guess := strings.ToLower(req.View.State.Values["guess"]["letters"].Value)
	view := updateModal(req)

	meta := strings.Split(req.View.PrivateMetadata, ",")
	gameId, err := primitive.ObjectIDFromHex(meta[0])

	if err != nil {
		fmt.Print("Could not convert hex string to mongo object id")
		res.WriteHeader(500)
		return
	}

	var game Game
	client.Collection("games").FindOne(context.TODO(), bson.D{{"_id", gameId}}).Decode(&game)

	user := req.User.Name

	var wordsFound []string
	if len(meta) > 1 {
		wordsFound = meta[1:]
	}

	incorrectGuess := true
	guessed := alreadyGuessed(wordsFound, guess)

	if !guessed {
		for _, word := range game.Words {
			if guess == word {
				wordsFound = append(wordsFound, word)
				incorrectGuess = false
				break
			}
		}
	}

	if guessed || incorrectGuess {
		view.Blocks = req.View.Blocks
		view.Blocks.BlockSet = req.View.Blocks.BlockSet[:2]
		view.Blocks.BlockSet = append(view.Blocks.BlockSet, gameInput(meta[0]))
		view.CallbackID = "play"
		view.PrivateMetadata = req.View.PrivateMetadata

		errorMessage := "*_" + strings.ToUpper(guess) + "_*"
		if guessed {
			errorMessage = errorMessage + " already guessed!"
		} else if incorrectGuess {
			errorMessage = errorMessage + " is incorrect!"
		}

		errorBlock := slack.NewTextBlockObject("mrkdwn", errorMessage, false, false)
		errorSection := slack.NewSectionBlock(errorBlock, nil, nil)

		view.Blocks.BlockSet = append(view.Blocks.BlockSet, errorSection)
		view.Blocks.BlockSet = append(view.Blocks.BlockSet, foundWordsSection(wordsFound)...)

		apiRes, err := api.UpdateView(view, view.ExternalID, req.Hash, req.View.ID)

		if err != nil {
			fmt.Printf("%+v", apiRes)
		}
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

		apiRes, err := api.UpdateView(view, "", req.Hash, req.View.ID)

		if err != nil {
			fmt.Printf("%+v", apiRes)
		}

		leaderboard := bson.D{{
			"$addToSet", bson.D{{
				"leaderboard", Leaderboard{
					User: user,
					Date: time.Now(),
				},
			}},
		}}

		_, err = client.Collection("games").UpdateByID(context.TODO(), game.Id, leaderboard)

		if err != nil {
			fmt.Printf("%+v", err)
			return
		}
	} else if len(wordsFound) > 0 {
		view.PrivateMetadata = strings.Join(wordsFound, ",")
		view.PrivateMetadata = meta[0] + "," + view.PrivateMetadata
		view.CallbackID = "play"
		view.Blocks = req.View.Blocks
		view.Blocks.BlockSet = req.View.Blocks.BlockSet[:2]
		view.Blocks.BlockSet = append(view.Blocks.BlockSet, gameInput(meta[0]))

		totalWords := strconv.Itoa(len(game.Words) - len(wordsFound))

		headerText := totalWords + " words left!"
		header := slack.NewTextBlockObject("plain_text", headerText, false, false)
		headerSection := slack.NewSectionBlock(header, nil, nil)

		view.Blocks.BlockSet[1] = headerSection

		view.Blocks.BlockSet = append(view.Blocks.BlockSet, foundWordsSection(wordsFound)...)

		apiRes, err := api.UpdateView(view, req.View.ExternalID, req.Hash, req.View.ID)

		if err != nil {
			fmt.Printf("%+v", apiRes)
			return
		}
	}
}

func findGameModal(res http.ResponseWriter, user string, private bool) (slack.ModalViewRequest, []Game) {
	games := getGames(res, bson.E{})
	if private {
		games = getOwnPrivateGames(games, user)
	} else {
		games = getActiveGames(games, user)
	}

	firstname, _, _ := getUser(user)

	var view slack.ModalViewRequest

	view.CallbackID = "find"
	view.Type = slack.ViewType("modal")
	view.Title = slack.NewTextBlockObject("plain_text", "Choose a game", false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Cancel", false, false)

	message := "Hey there " + firstname + "!  Choose a game from the options below to start playing!"

	header := slack.NewTextBlockObject("mrkdwn", message, false, false)
	headerSection := slack.NewSectionBlock(header, nil, nil)

	gameOptions := addGameOptions(0, games)

	view.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			slack.NewDividerBlock(),
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

func Instructions(triggerID string, res http.ResponseWriter, push bool) {
	var view slack.ModalViewRequest
	view.Close = slack.NewTextBlockObject("plain_text", "Back", false, false)
	view.Type = slack.ViewType("modal")
	view.Title = slack.NewTextBlockObject("plain_text", "Angrms Tips", false, false)

	if !push {
		view.Close.Text = "Close"
	}

	createHeader := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Creating a game", false, false))
	createMessage := "1. Provide some letters to create the game with.\n\n2. Duplicate letters aren't necessary.  The game will use the supplied letters multiple times if it can.\n\n3. If you set an expiration on the game it will only be playable for that amount of time.\n\n4. Units for setting an expiration are `m`, `h`, and `d`.  At this time those are the only ones supported.  The unit is preceded by a number, so setting it to `30m`, for example, would make the game inactive after 30 minutes.\n\n5. If you mark a game as private it will only be playable by you.\n\n5. You will *_only_* be shown the amount of words that are created but *_not_* the words themselves."
	createBlock := slack.NewTextBlockObject("mrkdwn", createMessage, false, false)
	createSection := slack.NewSectionBlock(createBlock, nil, nil)

	playHeader := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "How to play", false, false))
	playMessage := "1. You will guess one word at a time\n\n2. All games created will potentially have any of the letters used multiple times in the words.\n\n3. If a word is correct it will show up at the bottom\n\n4. If a guess is incorrect, nothing will happen and you need to manually clear the guess\n\n5. When you find all the words in a game you will be added to that game's leaderboard.\n\n6. Have fun! :confetti_ball:"
	playBlock := slack.NewTextBlockObject("plain_text", playMessage, false, false)
	playSection := slack.NewSectionBlock(playBlock, nil, nil)

	view.Blocks.BlockSet = []slack.Block{
		createHeader,
		createSection,
		slack.NewDividerBlock(),
		playHeader,
		playSection,
	}

	var (
		apiRes *slack.ViewResponse
		err    error
	)

	if push {
		apiRes, err = api.PushView(triggerID, view)
	} else {
		apiRes, err = api.OpenView(triggerID, view)
	}

	if err != nil {
		fmt.Printf("%+v", apiRes)
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

	welcome := ":wave: " + firstname + "!  If you haven't played yet, here are some pointers to get you started."
	messageBlock := slack.NewTextBlockObject("plain_text", welcome, false, false)
	messageSection := slack.NewSectionBlock(messageBlock, nil, nil)

	tipsButtonMessage := slack.NewTextBlockObject("plain_text", "Learn to play", false, false)
	tipsButtonText := slack.NewTextBlockObject("plain_text", "Tips", false, false)
	tipsButton := slack.NewButtonBlockElement("tips", "tips", tipsButtonText)
	tipsButtonAccessory := slack.NewAccessory(tipsButton)
	tipsButtonSection := slack.NewSectionBlock(tipsButtonMessage, nil, tipsButtonAccessory)
	tipsButtonSection.BlockID = "tips"

	callToAction := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Go ahead and create a game or find one to play :point_down::point_down::point_down:", false, false))

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
		tipsButtonSection,
		slack.NewDividerBlock(),
		callToAction,
		createButtonSection,
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
	case "play", "play-private":
		private := false

		if selectedOption == "play-private" {
			private = true
		}

		modal, games := findGameModal(res, req.User.Name, private)
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
	case "tips":
		Instructions(req.TriggerID, res, true)
		return
	}

	apiRes, err := api.PushView(req.TriggerID, view)

	if err != nil {
		fmt.Printf("%+v", apiRes)
		return
	}
}

func StatsInitView(res http.ResponseWriter, triggerID string, push bool) {
	games := addGameOptions(0, getGames(res, bson.E{}))
	var view slack.ModalViewRequest
	view.Type = slack.ViewType("modal")
	view.CallbackID = "gamestats"

	view.Title = slack.NewTextBlockObject("plain_text", "Angrms Stats", false, false)
	view.Close = slack.NewTextBlockObject("plain_text", "Close", false, false)

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
	gameID, _ := primitive.ObjectIDFromHex(req.ActionCallback.BlockActions[0].SelectedOption.Value)
	var game Game
	client.Collection("games").FindOne(context.TODO(), bson.E{"_id", gameID}).Decode(&game)
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

// func getStats(res http.ResponseWriter) []Stats {
// 	file, err := os.ReadFile("stats.json")

// 	if err != nil {
// 		res.Write([]byte("Could not open stats file"))
// 		return nil
// 	}

// 	var stats []Stats
// 	json.Unmarshal(file, &stats)

// 	return stats
// }

// func GamesSolvedStats(req slack.InteractionCallback, res http.ResponseWriter){
// 	stats := os.ReadFile("stats.json")
// 	var view slack.ModalViewRequest

// 	header := slack.NewTextBlockObject("plain_text", "Most Games Solved", false, false)
// 	headerBlock := slack.NewHeaderBlock(header, nil)

// 	var stats []slack.Block

// 	for _, user := range  {

// 	}
// }
