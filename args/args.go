package args

import "gitlab.sweetwater.com/mike_mayo/slackbot/slices"

func CheckArgs(args []string) {
	switch args[0] {
	case "create":
		createGame(args[1])
	case "play":
		playGame(args[1])
	}
}

func createGame(letters string) {
	slices.FindWordsWithLetters(letters)
}

func playGame(word string) {

}
