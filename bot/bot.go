package bot

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"os"
	"strings"
)

type Controller interface {
	Actions() [][]string
	Action(action string) error
}

func Run(control Controller) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	var ucfg tgbotapi.UpdateConfig = tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60
	updates, err := bot.GetUpdatesChan(ucfg)
	if err != nil {
		log.Panic(err)
	}

	const rowSize = 3
	var (
		buttons [][]tgbotapi.KeyboardButton
		row     []tgbotapi.KeyboardButton
	)
	for _, list := range control.Actions() {
		for _, act := range list {
			row = append(row, tgbotapi.NewKeyboardButton(act))
			if len(row) == rowSize {
				buttons = append(buttons, row)
				row = make([]tgbotapi.KeyboardButton, 0)
			}
		}
		if len(row) != 0 {
			buttons = append(buttons, row)
			row = make([]tgbotapi.KeyboardButton, 0)
		}
	}
	keyboard := tgbotapi.NewReplyKeyboard(buttons...)

	for {
		select {
		case update := <-updates:
			username := update.Message.From.UserName
			chatId := update.Message.Chat.ID
			text := strings.TrimSpace(update.Message.Text)

			log.Printf("[%s] %d %s", username, chatId, text)
			err = control.Action(text)
			var reply string
			if err == nil {
				reply = "играю " + text
			} else {
				reply = err.Error()
			}
			msg := tgbotapi.NewMessage(chatId, reply)
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
		}
	}
}
