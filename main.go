package main

import (
	"ReviewGenerator/reviewer"
	"ReviewGenerator/translator"
	"ReviewGenerator/utils"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"
)

func init() {
	logrus.SetFormatter(&utils.Formatter{})
	logrus.SetReportCaller(true)
}

type State int

const (
	NoReview State = iota
	NameAndAddress
	Category
	SubCategory
	Features
)

type Ctx struct {
	Review          reviewer.Review
	State           State
	ReviewMessageId int
	CurrentFeature  *reviewer.Feature
}

func MainHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := w.Write([]byte("Bot is running"))
	if err != nil {
		logrus.Error(err)
	}
}

func main() {
	http.HandleFunc("/", MainHandler)
	go func() {
		if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
			logrus.Fatal(err)
		}
	}()

	rc, err := reviewer.NewReviewCore("reviewer/reviewCore.json")
	if err != nil {
		logrus.Fatal(err)
	}

	token := os.Getenv("TOKEN")
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logrus.Fatal(err)
	}

	//bot.Debug = true
	logrus.Infof("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	/*
		https://api.telegram.org/botТОКЕН_БОТА/setWebhook?url=https://ИМЯ_HEROKU_ПРИЛОЖЕНИЯ.herokuapp.com/ТОКЕН_БОТА
	*/
	updates := bot.ListenForWebhook("/" + bot.Token)
	//updates, err := bot.GetUpdatesChan(u)
	//if err != nil {
	//	logrus.Fatal(err)
	//}

	var ctx = make(map[int64]*Ctx)

	var categoryKeyboard = tgbotapi.NewReplyKeyboard()
	var subCategoryKeyboards = make(map[string]tgbotapi.ReplyKeyboardMarkup)

	for _, category := range rc.Categories {
		categoryKeyboard.Keyboard = append(categoryKeyboard.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(strings.Title(category.Name))))

		sbc := tgbotapi.NewReplyKeyboard()
		for _, subCategory := range category.SubCategories {
			sbc.Keyboard = append(sbc.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(strings.Title(subCategory.String()))))
		}
		sbc.Keyboard = append(sbc.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Back")))

		subCategoryKeyboards[category.Name] = sbc
	}

	categoryKeyboard.Keyboard = append(categoryKeyboard.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Cancel")))

	var featuresKeyboard = tgbotapi.NewReplyKeyboard()
	var fieldKeyboards = make(map[string]tgbotapi.ReplyKeyboardMarkup)

	for _, feature := range rc.Features {
		featuresKeyboard.Keyboard = append(featuresKeyboard.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(feature.Name)))

		f := tgbotapi.NewReplyKeyboard()
		for _, field := range feature.Fields {
			f.Keyboard = append(f.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(strings.Title(field.String()))))
		}

		f.Keyboard = append(f.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Back")))

		fieldKeyboards[feature.Name] = f
	}

	featuresKeyboard.Keyboard = append(featuresKeyboard.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Back"), tgbotapi.NewKeyboardButton("Done")))

	for update := range updates {
		if update.Message != nil {
			text := update.Message.Text
			chatID := update.Message.Chat.ID

			if text == "Cancel" || (update.Message.IsCommand() && update.Message.Command() == "cancel") {
				ctx[chatID] = nil
				msg := tgbotapi.NewMessage(chatID, "Canceled")
				msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

				_, err := bot.Send(msg)
				if err != nil {
					logrus.Error(err)
				}
				continue
			}

			if strings.Index(text, "https://maps.app.goo.gl/") != -1 && (ctx[chatID] == nil || ctx[chatID].State == NoReview) {
				lines := strings.Split(text, "\n")
				if len(lines) < 2 {
					continue
				}

				name := lines[0]
				addressFull := lines[1]
				address := addressFull[:strings.Index(addressFull, ", Lviv")]

				r := reviewer.NewReview(rc)

				r.SetName(name)
				r.SetAddress(address)

				ctx[chatID] = &Ctx{
					Review: *r,
					State:  NameAndAddress,
				}

				msgText := name + "\n" + address + "\n"

				msg := tgbotapi.NewMessage(chatID, msgText)
				msg.ReplyToMessageID = update.Message.MessageID
				msg.ReplyMarkup = categoryKeyboard

				sentMsg, err := bot.Send(msg)
				if err != nil {
					logrus.Error(err)
				}

				ctx[chatID].ReviewMessageId = sentMsg.MessageID
				continue
			}

			if ctx[chatID] != nil && ctx[chatID].State == NameAndAddress {
				category := rc.GetCategoryWithName(strings.ToLower(text))
				if category != nil {
					if err = ctx[chatID].Review.SetCategory(*category); err == nil {
						ctx[chatID].State = Category
						msg := tgbotapi.NewMessage(chatID, "Category set: "+category.Name)
						msg.ReplyMarkup = subCategoryKeyboards[category.Name]

						_, err := bot.Send(msg)
						if err != nil {
							logrus.Error(err)
						}
					} else {
						logrus.Error(err)
						msg := tgbotapi.NewMessage(chatID, "Something went wrong, try again")
						msg.ReplyToMessageID = ctx[chatID].ReviewMessageId
						msg.ReplyMarkup = categoryKeyboard

						_, err := bot.Send(msg)
						if err != nil {
							logrus.Error(err)
						}
					}
				} else {
					msg := tgbotapi.NewMessage(chatID, "Select category")
					msg.ReplyToMessageID = ctx[chatID].ReviewMessageId
					msg.ReplyMarkup = categoryKeyboard

					_, err := bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}
				}
				continue
			}

			if ctx[chatID] != nil && ctx[chatID].State == Category {
				if text == "Back" {
					ctx[chatID].State = NameAndAddress
					msg := tgbotapi.NewMessage(chatID, "Back")
					msg.ReplyMarkup = categoryKeyboard

					_, err := bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}
					continue
				}

				subCategory := ctx[chatID].Review.Category().GetSubCategoryWithName(strings.ToLower(text))
				if subCategory != nil {
					if err = ctx[chatID].Review.SetSubCategory(*subCategory); err == nil {
						ctx[chatID].State = SubCategory
						msg := tgbotapi.NewMessage(chatID, "Sub category set: "+subCategory.String())
						msg.ReplyMarkup = featuresKeyboard

						_, err := bot.Send(msg)
						if err != nil {
							logrus.Error(err)
						}
					} else {
						logrus.Error(err)
						msg := tgbotapi.NewMessage(chatID, "Something went wrong, try again")
						msg.ReplyToMessageID = ctx[chatID].ReviewMessageId
						msg.ReplyMarkup = subCategoryKeyboards[ctx[chatID].Review.Category().Name]

						_, err := bot.Send(msg)
						if err != nil {
							logrus.Error(err)
						}
					}
				}
				continue
			}

			if ctx[chatID] != nil && ctx[chatID].State == SubCategory {
				if text == "Back" {
					ctx[chatID].State = Category
					msg := tgbotapi.NewMessage(chatID, "Back")
					msg.ReplyMarkup = subCategoryKeyboards[ctx[chatID].Review.Category().Name]

					_, err := bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}
					continue
				}

				if text == "Done" {
					review := ctx[chatID].Review.GenerateReview()
					review = translator.TranslateText(review)

					msg := tgbotapi.NewMessage(chatID, "Review length: "+fmt.Sprint(utf8.RuneCountInString(review)))
					msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

					_, err := bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}

					msg = tgbotapi.NewMessage(chatID, "<pre>"+review+"</pre>")
					msg.ParseMode = tgbotapi.ModeHTML

					_, err = bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}

					ctx[chatID] = nil
					continue
				}

				feature := rc.GetFeatureWithName(text)
				if feature != nil {
					err = ctx[chatID].Review.AddFeature(*feature)
					if ctx[chatID].Review.HasFeature(*feature) || err == nil {
						ctx[chatID].State = Features
						msg := tgbotapi.NewMessage(chatID, "Features: "+feature.Name)
						msg.ReplyMarkup = fieldKeyboards[feature.Name]
						ctx[chatID].CurrentFeature = feature

						_, err := bot.Send(msg)
						if err != nil {
							logrus.Error(err)
						}
					} else {
						logrus.Error(err)
						msg := tgbotapi.NewMessage(chatID, "Something went wrong, try again")
						msg.ReplyToMessageID = ctx[chatID].ReviewMessageId
						msg.ReplyMarkup = featuresKeyboard

						_, err := bot.Send(msg)
						if err != nil {
							logrus.Error(err)
						}
					}
				} else {
					msg := tgbotapi.NewMessage(chatID, "Select feature")
					msg.ReplyToMessageID = ctx[chatID].ReviewMessageId
					msg.ReplyMarkup = featuresKeyboard

					_, err := bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}
				}
				continue
			}

			if ctx[chatID] != nil && ctx[chatID].State == Features {
				if text == "Back" {
					ctx[chatID].State = SubCategory
					msg := tgbotapi.NewMessage(chatID, "Back")
					msg.ReplyMarkup = featuresKeyboard

					_, err := bot.Send(msg)
					if err != nil {
						logrus.Error(err)
					}
					continue
				}

				ctx[chatID].CurrentFeature.ToggleField(reviewer.Field(strings.ToLower(text)))

				ctx[chatID].State = SubCategory
				msg := tgbotapi.NewMessage(chatID, "Another features")
				msg.ReplyMarkup = featuresKeyboard
				ctx[chatID].CurrentFeature = nil

				_, err := bot.Send(msg)
				if err != nil {
					logrus.Error(err)
				}
				continue
			}
		}
	}
}
