package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"party-bot/service"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

var (
	bot   *linebot.Client
	cache *service.Cache
)

func init() {
	var err error
	bot, err = linebot.New(
		os.Getenv("LINE_CHANNEL_SECRET"),
		os.Getenv("LINE_ACCESS_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}
	cache = service.NewCache()
}

// LineBotHandler 處理 Line Bot 訊息的 Handler
func LineBotHandler(c *gin.Context) {
	events, err := bot.ParseRequest(c.Request)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			c.String(http.StatusBadRequest, "Bad Request")
		} else {
			c.String(http.StatusInternalServerError, "Internal Server Error")
		}
		return
	}
	for _, event := range events {
		var userId string
		switch event.Source.Type {
		case linebot.EventSourceTypeUser:
			userId = event.Source.UserID
		case linebot.EventSourceTypeGroup:
			userId = event.Source.GroupID
		case linebot.EventSourceTypeRoom:
			userId = event.Source.RoomID
		}
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if message.Text == "［愛的留言］" {
					cache.Set(userId+"Danmaku", true, 60*time.Second)
					if _, err := bot.ReplyMessage(
						event.ReplyToken,
						linebot.NewTextMessage("感謝您使用此功能：\n請在接下來的5分鐘內，將您想告訴新人的話傳給我～\n\n愛的留言就會投射至大螢幕\n～趕快留言給新人吧(๑ ◡ ๑)"),
					).Do(); err != nil {
						log.Print(err)
					}
				} else if message.Text == " [拍立得列印] " {
					cache.Set(userId+"Photo", true, 300*time.Second)
					if _, err := bot.ReplyMessage(
						event.ReplyToken,
						linebot.NewTextMessage("感謝您使用此功能：\n請在接下來的5分鐘內，將希望列印的照片上傳給我～\n數量有限，印完為止，可以到入口處看看您的照片有沒有印出來唷(๑•̀ㅂ•́)و✧~"),
					).Do(); err != nil {
						log.Print(err)
					}
				} else {
					if _, ok := cache.Get(userId + "Danmaku"); ok {
						handleDanmakuMessage(message.Text, userId, event.ReplyToken)
						// if _, err := bot.ReplyMessage(
						// 	event.ReplyToken,
						// 	linebot.NewTextMessage(message.Text),
						// ).Do(); err != nil {
						// 	log.Print(err)
						// }
						return
					}
				}
			case *linebot.ImageMessage:
				if _, ok := cache.Get(userId + "Photo"); ok {
					handleImageMessage(bot, event.ReplyToken, message, userId)
				}
			}
		}
	}

	c.String(http.StatusOK, "OK")
}

func handleImageMessage(bot *linebot.Client, replyToken string, message *linebot.ImageMessage, userId string) {

	// 下載圖片
	content, err := bot.GetMessageContent(message.ID).Do()
	if err != nil {
		log.Print(err)
		return
	}
	defer content.Content.Close()

	// 將圖片保存到本地文件系統
	filePath := service.SaveImageFileLocally(content.Content, message.ID)
	if filePath == "" {
		log.Println("Failed to save the image locally")
		return
	}

	// 在這裡你可以對本地保存的圖片進行進一步的處理
	senderProfile, err := bot.GetProfile(userId).Do()
	if err != nil {
		log.Printf("Error getting sender's profile: %v", err)
		// 錯誤處理...
		return
	}
	senderName := senderProfile.DisplayName
	newImage := service.SaveImage(senderName, filePath)
	// 回覆用戶
	if _, err := bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(fmt.Sprintf("已收到您的圖片，您的照片序號是：%s", newImage.Serial)),
	).Do(); err != nil {
		log.Print(err)
	}
}

func handleDanmakuMessage(message string, userId string, replyToken string) {
	senderProfile, err := bot.GetProfile(userId).Do()
	if err != nil {
		log.Printf("Error getting sender's profile: %v", err)
		// 錯誤處理...
		return
	}
	senderName := senderProfile.DisplayName
	regex := regexp.MustCompile(`^[\p{Han}\p{Katakana}\p{Hiragana}\p{Hangul}a-zA-Z0-9\s]+$`)
	if !regex.MatchString(message) || utf8.RuneCountInString(message) > 20 {
		if _, err := bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage("只能傳20個以內的訊息唷~"),
		).Do(); err != nil {
			log.Print(err)
		}
		return
	}
	BroadcastMessage(message)
	service.SaveMessage(message, senderName)
}
