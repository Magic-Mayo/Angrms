package slackHandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"gitlab.sweetwater.com/mike_mayo/slackbot/args"
)

var err = godotenv.Load(".env")
var signing_secret string = os.Getenv("SIGNING_SECRET")
var api = slack.New(os.Getenv("OAUTH_TOKEN"), slack.OptionDebug(true))

func verifySlack(req *http.Request) error {
	verifier, err := slack.NewSecretsVerifier(req.Header, signing_secret)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	verifier.Write(body)
	if err = verifier.Ensure(); err != nil {
		fmt.Println(err.Error())
		return err
	}

	return nil
}

func SlashCommandHandler(res http.ResponseWriter, req *http.Request) {
	err := verifySlack(req)

	if err != nil {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	command, err := slack.SlashCommandParse(req)

	switch command.Command {
	case "/angrms":
		args.CheckArgs(res, command)
	default:
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func InteractiveHandler(res http.ResponseWriter, req *http.Request) {
	err := verifySlack(req)
	if err != nil {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	var modalRes slack.InteractionCallback
	err = json.Unmarshal([]byte(req.FormValue("payload")), &modalRes)

	if err != nil {
		fmt.Printf("%+v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch modalRes.View.CallbackID {
	case "create":
		args.SaveNewGame(modalRes, res)
	case "play":
		args.PlayGame(modalRes, res)
	case "find":
		args.StartGame(modalRes, res)
	case "main":
		args.ParseMenu(modalRes, res)
	case "stats":
		args.StatsInitView(res, modalRes.TriggerID, true)
	case "gamestats":
		args.ShowStats(modalRes, res)
	default:
		res.WriteHeader(http.StatusInternalServerError)
	}
}
