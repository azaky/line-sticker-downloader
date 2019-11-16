package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/azaky/line-sticker-downloader/util"
	"github.com/line/line-bot-sdk-go/linebot"
)

const (
	targetExportDir = "/tmp/line-stickers-export"
)

var (
	patterns []string = []string{
		"https://line.me/S/sticker/(\\d+)/",
		"https://store.line.me/stickershop/product/(\\d+)/",
	}
)

type Bot struct {
	c       *linebot.Client
	host    string
	regexps []*regexp.Regexp
}

func NewBot(secret, token, host string) *Bot {
	l, err := linebot.New(secret, token)
	if err != nil {
		log.Fatalf("Error when initializing linebot: %s", err.Error())
	}
	regexps := make([]*regexp.Regexp, 0)
	for _, pattern := range patterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Error compiling pattern [%s]: %s", pattern, err.Error())
		}
		regexps = append(regexps, regex)
	}
	return &Bot{l, host, regexps}
}

func (bot *Bot) Handler(w http.ResponseWriter, req *http.Request) {
	events, err := bot.c.ParseRequest(req)
	if err != nil {
		log.Printf("Error in ParseRequest: %s", err.Error())
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	go bot.processEvents(events)
	w.WriteHeader(200)
}

func (bot *Bot) reply(event *linebot.Event, messages ...string) error {
	var lineMessages []linebot.SendingMessage
	for _, message := range messages {
		lineMessages = append(lineMessages, linebot.NewTextMessage(message))
	}
	_, err := bot.c.ReplyMessage(event.ReplyToken, lineMessages...).Do()
	if err != nil {
		log.Printf("Error replying to %+v: %s", event.Source, err.Error())
	}
	return err
}

func (bot *Bot) processEvents(events []*linebot.Event) {
	for _, event := range events {
		log.Printf("[EVENT][%s] Source: %#v", event.Type, event.Source)
		if event.Type != linebot.EventTypeMessage {
			bot.reply(event, "Please send me stickers or sticker shop URL!")
			return
		}
		switch event.Message.(type) {
		case *linebot.StickerMessage:
			bot.processStickerMessage(event)
		case *linebot.TextMessage:
			bot.processTextMessage(event)
		default:
			bot.reply(event, "Please send me stickers or sticker shop URL!")
		}
	}
}

func (bot *Bot) processStickerMessage(event *linebot.Event) {
	sticker, ok := event.Message.(*linebot.StickerMessage)
	if !ok {
		bot.reply(event, "Please send me stickers or sticker shop URL!")
		return
	}

	err := bot.processSticker(sticker.PackageID)

	if err != nil {
		log.Printf("Error when processing sticker: %s", err.Error())
		bot.reply(event, fmt.Sprintf("An error occured when processing your stickers: %s", err.Error()))
	} else {
		bot.reply(
			event,
			"Copy the link below and open it in your browser (because Line browser prevents downloading files)",
			fmt.Sprintf("%s/export/%s.zip", bot.host, sticker.PackageID))
	}
}

func (bot *Bot) findStickerID(text string) string {
	for _, regex := range bot.regexps {
		matches := regex.FindStringSubmatch(text)
		if matches == nil {
			continue
		}
		return matches[1]
	}
	return ""
}

func (bot *Bot) processTextMessage(event *linebot.Event) {
	message, ok := event.Message.(*linebot.TextMessage)
	if !ok {
		bot.reply(event, "Please send me stickers or sticker shop URL!")
		return
	}

	log.Printf("text message: %#v", message)
	id := bot.findStickerID(message.Text)
	if id == "" {
		bot.reply(event, "Please send me stickers or sticker shop URL!")
		return
	}

	err := bot.processSticker(id)

	if err != nil {
		log.Printf("Error when processing sticker: %s", err.Error())
		bot.reply(event, fmt.Sprintf("An error occured when processing your stickers: %s", err.Error()))
	} else {
		bot.reply(
			event,
			"Copy the link below and open it in your browser (because Line browser prevents downloading files)",
			fmt.Sprintf("%s/export/%s.zip", bot.host, id))
	}
}

func (bot *Bot) processSticker(id string) error {
	log.Printf("Processing stickers with id %s ...", id)

	// Check existing file
	targetExport := targetExportDir + "/" + id + ".zip"
	if _, err := os.Stat(targetExport); err == nil {
		log.Printf("Sticker with id %s already exists.", id)
		return nil
	}

	// TODO: mutex by id

	target := fmt.Sprintf("/tmp/%s", id)
	rawFilename := fmt.Sprintf("/tmp/%s-raw.zip", id)

	// cleanup
	defer func() {
		log.Printf("Finish processing stickers with id %s. Cleaning up ...", id)
		if e := exec.Command("rm", "-r", target, rawFilename).Run(); e != nil {
			log.Printf("Error running rm: %s", e.Error())
		}
	}()

	// Download and unzip
	log.Printf("Downloading stickers with id %s ...", id)
	err := util.Download(rawFilename, fmt.Sprintf("http://dl.stickershop.line.naver.jp/products/0/0/1/%s/iphone/stickers@2x.zip", id))
	if err != nil {
		log.Printf("Error downloading stickers: %s", err.Error())
		return fmt.Errorf("stickers cant be downloaded")
	}
	if e := os.Mkdir(target, os.FileMode(0755)); e != nil && !os.IsExist(e) {
		log.Printf("Error running mkdir: %s", e.Error())
		return fmt.Errorf("internal error")
	}
	if e, stderr := util.Exec("unzip", rawFilename, "-d", target); e != nil {
		log.Printf("Error running unzip: %s, stderr: %s", e.Error(), stderr)
		return fmt.Errorf("stickers cant be unzipped")
	}

	// Get sticker name
	stickerName := id
	rawMeta, err := ioutil.ReadFile(target + "/productInfo.meta")
	if err != nil {
		log.Printf("Error reading metadata: %s", err.Error())
	} else {
		var meta struct {
			Title map[string]string `json:"title"`
		}
		if e := json.Unmarshal(rawMeta, &meta); e != nil {
			log.Printf("Error unmarshaling metadata: %s", err.Error())
		} else {
			if _, ok := meta.Title["en"]; ok {
				stickerName = meta.Title["en"]
			}
		}
	}
	log.Printf("StickerName: %s", stickerName)

	files, err := filepath.Glob(target + "/*key*")
	if err != nil {
		log.Printf("Error running glob: %s", err.Error())
	}
	files = append(files, target+"/productInfo.meta", target+"/tab_off@2x.png", target+"/tab_on@2x.png")
	if e, stderr := util.Exec("rm", files...); e != nil {
		log.Printf("Error running rm: %s, stderr: %s", e.Error(), stderr)
		return fmt.Errorf("internal error")
	}

	// make sticker pack with 20 items per folder
	dir, err := os.Open(target)
	if err != nil {
		log.Fatalf("Error opening target dir: %s", err.Error())
	}
	names, err := dir.Readdirnames(0)
	if err != nil {
		log.Fatalf("Error opening target dir: %s", err.Error())
	}
	sort.Strings(names)
	n := (len(names) + 19) / 20
	for i := 0; i < n; i++ {
		dirname := fmt.Sprintf("%s (%d:%d)", stickerName, i+1, n)
		if n == 1 {
			dirname = stickerName
		}
		if e := os.Mkdir(target+"/"+dirname, os.FileMode(0755)); e != nil {
			log.Printf("Error running mkdir %s: %s", dirname, e.Error())
			return fmt.Errorf("internal error")
		}
		start := i * 20
		end := (i + 1) * 20
		if end > len(names) {
			end = len(names)
		}
		args := make([]string, 0)
		for _, name := range names[start:end] {
			args = append(args, target+"/"+name)
		}
		args = append(args, target+"/"+dirname)
		if e, stderr := util.Exec("mv", args...); e != nil {
			log.Printf("Error running mv: %s, stderr: %s", e.Error(), stderr)
			return fmt.Errorf("internal error")
		}
	}
	if e, stderr := util.ExecDir(target, "zip", targetExportDir+"/"+id, ".", "-r"); e != nil {
		log.Printf("Error running zip: %s, stderr: %s", err.Error(), stderr)
		return fmt.Errorf("stickers cant be zipped")
	}
	return nil
}

func main() {
	channelToken := os.Getenv("CHANNEL_TOKEN")
	if channelToken == "" {
		log.Fatalf("CHANNEL_TOKEN env is required")
	}
	channelSecret := os.Getenv("CHANNEL_SECRET")
	if channelSecret == "" {
		log.Fatalf("CHANNEL_SECRET env is required")
	}
	host := os.Getenv("HOST")
	if host == "" {
		log.Fatalf("HOST env required")
	}
	bot := NewBot(channelSecret, channelToken, host)
	http.HandleFunc("/callback", bot.Handler)

	// Setup root endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("Hello from sticker downloader"))
	})

	// Setup file server
	if e := os.Mkdir(targetExportDir, os.FileMode(0755)); e != nil && !os.IsExist(e) {
		log.Fatalf("Error running mkdir: %s", e.Error())
	}
	http.Handle("/export/", http.StripPrefix("/export/", http.FileServer(http.Dir(targetExportDir))))

	log.Printf("Starting server at port 8100...")
	if err := http.ListenAndServe(":8100", nil); err != nil {
		log.Fatal("Error listening to 8100: %s", err.Error())
	}
}
