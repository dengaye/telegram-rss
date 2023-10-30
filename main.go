package main

import (
	"encoding/json"
	"flag"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mmcdole/gofeed"
	"os"
	"time"
)

// 基础环境配置
var BotToken *string
var WeeklyChannelID *int64
var NewsChannelID *int64
var BlogsChannelID *int64

func TokenValid() {
	if *BotToken == "" || *WeeklyChannelID == 0 || *NewsChannelID == 0 || *BlogsChannelID == 0 {
		panic("BotToken && ChannelId cannot be empty")
	}
}

func init() {
	BotToken = flag.String("tg_bot", "", "Telegram bot token")
	WeeklyChannelID = flag.Int64("tg_weekly_channel", 0, "Telegram weekly channel id")
	NewsChannelID = flag.Int64("tg_news_channel", 0, "Telegram news channel id")
	BlogsChannelID = flag.Int64("tg_blogs_channel", 0, "Telegram blogs channel id")
	flag.Parse()
	TokenValid()
	getAllRssInfo()
}

// RSS 构成阶段
type RSSInfos struct {
	RssInfo []RssInfo `json:"rss_info"`
}

type RssInfo struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	FullContent bool   `json:"full_content"`
}

var WeeklyRssInfos = RSSInfos{nil}
var NewsRssInfos = RSSInfos{nil}
var BlogsRssInfos = RSSInfos{nil}

// 从 配置文件中获取 rss 链接
// 根据 rss 链接获取更新
func GetRssInfo(filePath string, RssInfos *RSSInfos) {
	rssFile, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	err = json.NewDecoder(rssFile).Decode(RssInfos)
	// fmt.Printf("RssInfos: %v\n", WeeklyRssInfos)
	if err != nil {
		panic(err)
	}

}

func getAllRssInfo() {
	GetRssInfo("./rss/weekly.json", &WeeklyRssInfos);
	GetRssInfo("./rss/news.json", &NewsRssInfos);
	GetRssInfo("./rss/blogs.json", &BlogsRssInfos);
}

func GetAllPosts() {
	GetPosts(WeeklyRssInfos, WeeklyChannelID)
	GetPosts(NewsRssInfos, NewsChannelID)
	GetPosts(BlogsRssInfos, BlogsChannelID)
}

// 根据时间筛选昨天一整天的文章
func GetPosts(RssInfos RSSInfos, ChannelId *int64) {
	// 
	var msg = make([]string, 0)
	for _, info := range RssInfos.RssInfo {
		msg = append(msg, GetPostInfo(info)...)
	}
	PushPost(msg, ChannelId)
}

func getDatetime(times ...*time.Time) *time.Time {
	for _, d := range times {
		if d != nil && !d.IsZero() {
			return d
		}
	}
	return times[len(times)-1]
}

func GetPostInfo(rss RssInfo) []string {
	var msg = make([]string, 0)
	now := time.Now().UTC()
	startTime := now.Add(-24 * time.Hour)
	start := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), startTime.Hour(), 0, 0, 0, now.Location()).Unix()
	end := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Unix()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rss.Url)
	if err != nil {
		fmt.Print(err.Error())
	} else {
		for _, item := range feed.Items {
			parseDatetime := getDatetime(item.PublishedParsed, item.UpdatedParsed)
			if parseDatetime != nil && parseDatetime.Unix() >= start && parseDatetime.Unix() < end {
				msgItem := fmt.Sprintln(item.Title, item.Link)
				msg = append(msg, msgItem)

			}
		}
	}

	return msg
}

// 从配置文件获取推送方式
// 使用对应的推送渠道推送文章
func PushPost(msg []string, ChannelId *int64) {
	bot, err := tgbotapi.NewBotAPI(*BotToken)
	if err != nil {
		panic(err)
	}
	bot.Debug = true
	for _, s := range msg {
		_, err = bot.Send(tgbotapi.NewMessage(*ChannelId, "test"))
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	GetAllPosts()
}
