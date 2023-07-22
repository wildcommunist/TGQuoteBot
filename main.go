package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Entrio/subenv"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

var (
	botToken     = "NOT_A_TOKEN"
	botURL       = ""
	fileLocation = ""
	quotes       []string
)

type (
	Chat struct {
		Id int `json:"id"`
	}

	Message struct {
		Text string `json:"text"`
		Chat Chat   `json:"chat"`
	}

	Update struct {
		UpdateId int     `json:"update_id"`
		Message  Message `json:"message"`
	}
)

// Define a few constants and variable to handle different commands
const punchCommand string = "/doyouknow"

var lenPunchCommand int = len(punchCommand)

const startCommand string = "/start"

var lenStartCommand int = len(startCommand)

const botTag string = "@I_can_has_quote_bot"

var lenBotTag int = len(botTag)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	debug := flag.Bool("debug", false, "sets log level to debug")
	jsonLogger := flag.Bool("json", false, "sets log level to debug")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if !*jsonLogger {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Debug().Msg("Debug logging enabled.")

	botToken = subenv.Env("BOT_TOKEN", "")

	if len(botToken) == 0 {
		log.Fatal().Msg("BOT_TOKEN cannot be empty!")
	}

	botURL = fmt.Sprintf("https://api.telegram.org/bot%s", botToken)
	log.Debug().Str("token", botToken).Str("url", botURL).Msg("Variables")

	fileLocation = subenv.Env("QUOTES_FILE_URI", "./samples.txt")
	log.Debug().Str("quotes", fileLocation).Msg("quotes file location")

	// Read number of lines in the file
	f, err := os.Open(fileLocation)
	if err != nil {
		log.Err(err).Str("location", fileLocation).Msg("failed to open file with quotes")
		os.Exit(1)
	}

	// Read each line into string array
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Append line to result.
		quotes = append(quotes, line)
	}
	log.Debug().Int("lines", len(quotes)).Msg("quotes loaded")

	router := mux.NewRouter()
	router.HandleFunc("/", HandleTelegramWebHook)
	log.Info().Str("port", subenv.Env("LISTEN_PORT", ":1323")).Msg("Server started")
	http.ListenAndServe(subenv.Env("LISTEN_PORT", ":1323"), router)
}

func parseTelegramRequest(r *http.Request) (*Update, error) {
	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Err(err).Msg("could not decode incoming update")
		return nil, err
	}
	return &update, nil
}

func sanitize(s string) string {
	if len(s) >= lenStartCommand {
		if s[:lenStartCommand] == startCommand {
			s = s[lenStartCommand:]
		}
	}

	if len(s) >= lenPunchCommand {
		if s[:lenPunchCommand] == punchCommand {
			s = s[lenPunchCommand:]
		}
	}
	if len(s) >= lenBotTag {
		if s[:lenBotTag] == botTag {
			s = s[lenBotTag:]
		}
	}
	return s
}

// HandleTelegramWebHook sends a message back to the chat with a punchline starting by the message provided by the user.
func HandleTelegramWebHook(w http.ResponseWriter, r *http.Request) {

	// Parse incoming request
	var update, err = parseTelegramRequest(r)
	if err != nil {
		log.Err(err).Msg("error parsing update")
		return
	}

	// Sanitize input
	var sanitizedSeed = sanitize(update.Message.Text)

	// Call RapLyrics to get a punchline
	var lyric, errRandomQuote = getPunchline(sanitizedSeed)
	if errRandomQuote != nil {
		log.Err(errRandomQuote).Msg("got error while reading random quote")
		return
	}

	// Send the punchline back to Telegram
	var telegramResponseBody, errTelegram = sendTextToTelegramChat(update.Message.Chat.Id, lyric)
	if errTelegram != nil {
		log.Err(errTelegram).Str("response", telegramResponseBody).Msg("got error while sending data to telegram")
	} else {
		log.Info().Int("chatID", update.Message.Chat.Id).Msg("response sent")
	}
}

// getPunchline calls the RapLyrics API to get a punchline back.
func getPunchline(input string) (string, error) {
	quote := quotes[rand.Intn((len(quotes)-1)+1)]
	log.Debug().Str("quote", quote)
	return quote, nil
}

// sendTextToTelegramChat sends a text message to the Telegram chat identified by its chat Id
func sendTextToTelegramChat(chatId int, text string) (string, error) {

	log.Printf("Sending %s to chat_id: %d", text, chatId)
	response, err := http.PostForm(
		fmt.Sprintf("%s/sendMessage", botURL),
		url.Values{
			"chat_id": {strconv.Itoa(chatId)},
			"text":    {text},
		})

	if err != nil {
		log.Err(err).Msg("error sending response to chat")
		return "", err
	}
	defer response.Body.Close()

	var bodyBytes, errRead = ioutil.ReadAll(response.Body)
	if errRead != nil {
		log.Err(errRead).Msg("error p[arsing telegram answer")
		return "", err
	}
	bodyString := string(bodyBytes)
	log.Debug().Str("response", bodyString).Msg("telegram response")

	return bodyString, nil
}
