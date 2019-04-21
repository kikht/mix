package bot

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"os"
	"strings"

	"github.com/kikht/mix/controller"
)

func Run(control *controller.Controller) {
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
	for _, a := range control.Events() {
		row = append(row, tgbotapi.NewKeyboardButton(a))
		if len(row) == rowSize {
			buttons = append(buttons, row)
			row = make([]tgbotapi.KeyboardButton, 0)
		}
	}
	buttons = append(buttons, row)
	row = make([]tgbotapi.KeyboardButton, 0)
	for _, a := range control.Ambience() {
		row = append(row, tgbotapi.NewKeyboardButton(a))
		if len(row) == rowSize {
			buttons = append(buttons, row)
			row = make([]tgbotapi.KeyboardButton, 0)
		}
	}
	buttons = append(buttons, row)
	log.Println(buttons)
	keyboard := tgbotapi.NewReplyKeyboard(buttons...)
	log.Println(keyboard)

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
