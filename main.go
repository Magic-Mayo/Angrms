package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"gitlab.sweetwater.com/mike_mayo/slackbot/args"
	handleResponse "gitlab.sweetwater.com/mike_mayo/slackbot/response/handleResponse"
)

var err = godotenv.Load(".env")

var api = slack.New(os.Getenv("BOT_USER_OAUTH_TOKEN"), slack.OptionDebug(true))

type SlackRequest struct {
	Command string `json:"command"`
	Text    string `json:"text"`
	User    string `json:"user_name"`
	Url     string `json:"response_url"`
}

func handleEvents(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sv, err := slack.NewSecretsVerifier(r.Header, os.Getenv("SIGNING_SECRET"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		fmt.Println("[INFO] URL Verification message received")
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	}
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			api.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
		case *slackevents.MessageEvent:
			if ev.Text == "sweetwater" {
				fmt.Println("[info] kudos")
				api.PostMessage(ev.Channel, slack.MsgOptionText("Sweetwater rocks!", false))
			}
		}

	}
}

func slashCommandHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Println("[INFO]Got Slash Command")
	verifier, err := slack.NewSecretsVerifier(r.Header, os.Getenv("SIGNING_SECRET"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = ioutil.NopCloser(io.TeeReader(r.Body, &verifier))
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = verifier.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch s.Command {
	case "/angrm":

		response := "Finding words from "
		w.Write([]byte(response))

	default:
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func main() {
	HandleResponse := handleResponse.HandleResponse
	CheckArgs := args.CheckArgs

	// http.HandleFunc("/commands", slashCommandHandler)
	http.HandleFunc("/commands", func(res http.ResponseWriter, req *http.Request) {
		var parsed SlackRequest
		body := json.NewDecoder(req.Body)
		err := body.Decode(&parsed)

		params := strings.Fields(parsed.Text)

		if err != nil {
			HandleResponse(res, "Error decoding request", http.StatusBadRequest)
			return
		}

		CheckArgs(params)
		HandleResponse(res, parsed, 200)
	})

	port := os.Getenv("PORT")
	fmt.Println("Just felt like running.... http://localhost" + port)
	defer fmt.Println("I think I'll go home now")

	log.Fatal(http.ListenAndServe(port, nil))
}
