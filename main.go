package main

import (
	"encoding/json"
	"flag"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mmcdole/gofeed"
	"os"
	"time"
	"sync"
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
func GetRssInfo(filePath string, RssInfos *RSSInfos) error {
	rssFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer rssFile.Close()

	err = json.NewDecoder(rssFile).Decode(RssInfos)
	if err != nil {
		return fmt.Errorf("解析JSON失败: %v", err)
	}
	return nil
}

func getAllRssInfo() {
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := GetRssInfo("./rss/weekly.json", &WeeklyRssInfos); err != nil {
			fmt.Printf("获取weekly RSS信息失败: %v\n", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := GetRssInfo("./rss/news.json", &NewsRssInfos); err != nil {
			fmt.Printf("获取news RSS信息失败: %v\n", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := GetRssInfo("./rss/blogs.json", &BlogsRssInfos); err != nil {
			fmt.Printf("获取blogs RSS信息失败: %v\n", err)
		}
	}()

	wg.Wait()
}

func GetAllPosts() {
	var wg sync.WaitGroup

	wg.Add(3) // 设置 WaitGroup 的计数器为 3，因为我们有 3 个并发任务

	go func() {
		GetPosts(WeeklyRssInfos, WeeklyChannelID)
		// 当任务完成时，调用 Done 方法减少 WaitGroup 的计数器
		wg.Done() 
	}()

	go func() {
		GetPosts(NewsRssInfos, NewsChannelID)
		wg.Done()
	}()

	go func() {
		GetPosts(BlogsRssInfos, BlogsChannelID)
		wg.Done()
	}()

	wg.Wait() // 等待所有任务完成
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
	now := time.Now()
	// 获取最近24小时的更新
	oneDayAgo := now.Add(-24 * time.Hour)

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rss.Url)
	if err != nil {
		fmt.Printf("解析 RSS 失败 [%s]: %v\n", rss.Title, err)
		return msg
	}

	for _, item := range feed.Items {
		parseDatetime := getDatetime(item.PublishedParsed, item.UpdatedParsed)
		if parseDatetime == nil {
			continue
		}

		// 获取最近24小时的更新
		if parseDatetime.After(oneDayAgo) && parseDatetime.Before(now) {
			var msgItem string
			if rss.FullContent && item.Description != "" {
				msgItem = fmt.Sprintf("*%s*\n%s\n\n%s", 
					item.Title, 
					item.Description,
					item.Link)
			} else {
				msgItem = fmt.Sprintf("*%s*\n%s", 
					item.Title,
					item.Link)
			}
			msg = append(msg, msgItem)
		}
	}

	return msg
}

// 从配置文件获取推送方式
// 使用对应的推送渠道推送文章
func PushPost(msg []string, ChannelId *int64) {
	if len(msg) == 0 {
		return
	}

	bot, err := tgbotapi.NewBotAPI(*BotToken)
	if err != nil {
		fmt.Printf("创建 Bot 失败: %v\n", err)
		return
	}

	bot.Debug = false
	
	for _, s := range msg {
		message := tgbotapi.NewMessage(*ChannelId, s)
		message.ParseMode = "Markdown"
		
		// 添加重试机制
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			_, err = bot.Send(message)
			if err == nil {
				break
			}
			
			fmt.Printf("发送消息失败 (尝试 %d/%d): %v\n", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(time.Second * 3)
			}
		}
	}
}

func main() {
	fmt.Printf("开始执行任务: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	
	getAllRssInfo()
	GetAllPosts()
	fmt.Println("任务执行完成")
}
