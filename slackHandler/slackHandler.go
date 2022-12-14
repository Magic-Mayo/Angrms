package slackHandler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	handleResponse "gitlab.sweetwater.com/mike_mayo/slackbot/response"
)

var err = godotenv.Load(".env")
var signing_secret string = os.Getenv("SIGNING_SECRET")

type Element struct {
	Type      string
	Action_id string
}

type Text struct {
	Type string
	Text string
}

type Label struct {
	Type  string
	Text  string
	Emoji bool
}

type InputBlock struct {
	Type    string
	Element Element
	Label   Label
}

type Block struct {
	Type string
	Text Text
}

type Modal struct {
	Type   string
	Submit Label
	Close  Label
	Title  Label
	Blocks []interface{}
}

func verifySlack(req *http.Request, res http.ResponseWriter) (slack.SecretsVerifier, error) {
	verifier, err := slack.NewSecretsVerifier(req.Header, signing_secret)

	if err != nil {
		handleResponse.HandleResponse(res, "", http.StatusInternalServerError)
		return verifier, err
	}
	return verifier, err
}

func slashCommand(req *http.Request, res http.ResponseWriter, verifier slack.SecretsVerifier) (slack.SlashCommand, error) {
	req.Body = io.NopCloser(io.TeeReader(req.Body, &verifier))
	s, err := slack.SlashCommandParse(req)

	if err != nil {
		handleResponse.HandleResponse(res, "", http.StatusInternalServerError)
		return s, err
	}

	if err = verifier.Ensure(); err != nil {
		handleResponse.HandleResponse(res, "", http.StatusUnauthorized)
		return s, err
	}

	return s, err
}

func SlashCommandHandler(res http.ResponseWriter, req *http.Request) {
	verifier, er := verifySlack(req, res)
	command, err := slashCommand(req, res, verifier)

	if er != nil || err != nil {
		return
	}

	switch command.Command {
	case "/angrm":
		InteractiveHandler(res, command)
		// args.CheckArgs(res, strings.Fields(command.Text), command.UserName)
	default:
		handleResponse.HandleResponse(res, "", http.StatusInternalServerError)
		return
	}
}

func InteractiveHandler(res http.ResponseWriter /*req *http.Request*/, command slack.SlashCommand) {
	// verifier, er := verifySlack(req, res)
	// command, err := slashCommand(req, res, verifier)
	// fmt.Print(err)
	// if er != nil || err != nil {
	// 	return
	// }

	var modal Modal
	modal.Type = "modal"
	modal.Close = Label{
		Type:  "plain_text",
		Text:  "Cancel",
		Emoji: false,
	}
	modal.Submit = Label{
		Type:  "plain_text",
		Text:  "Submit",
		Emoji: false,
	}

	modal.Title = Label{
		Type:  "plain_text",
		Text:  "Angrms",
		Emoji: false,
	}

	user := strings.ToTitle(command.UserName)
	user = strings.Split(user, "_")[0]
	welcome := ":wave: " + user + "!  If you haven't played yet, here are some pointers to get you started.\n1. Provide some letters to create the game with.\n2. Duplicate letters aren't necessary.  The game will use the letters given multiple times if it can.\n3. You will *_only_* be shown the amount of words that are created but *_not_* the words themselves.\n4. Have fun!"

	modal.Blocks = append(modal.Blocks, Block{
		Type: "section",
		Text: Text{
			Type: "mrkdwn",
			Text: welcome,
		},
	})
	modal.Blocks = append(modal.Blocks, Block{
		Type: "divider",
	})
	modal.Blocks = append(modal.Blocks, InputBlock{
		Type: "input",
		Element: Element{
			Type:      "plain_text_input",
			Action_id: "plain_text_input-action",
		},
		Label: Label{
			Type:  "plain_text",
			Text:  "Letters to create game with",
			Emoji: true,
		},
	})

	jsonString, err := json.Marshal(modal)

	fmt.Print(jsonString)

	if err != nil {
		handleResponse.HandleResponse(res, "", http.StatusInternalServerError)
		return
	}
	handleResponse.HandleResponse(res, modal, 200)
}
