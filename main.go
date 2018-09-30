package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/go-resty/resty"
	"github.com/kovetskiy/ko"
)

var (
	version = "[manual build]"
	usage   = "ttshka " + version + os.ExpandEnv(`

Usage:
  ttshka [options] get
  ttshka [options] start <id>
  ttshka [options] sync
  ttshka [options] stop <id>
  ttshka -h | --help
  ttshka --version

Options:
  -h --help        Show this screen.
  --config <path>  Read config from here.
                    [default: $HOME/.config/ttshka.conf]
  --version        Show version.
`)
)

const (
	BaseURL    = "https://app.trackingtime.co"
	TimeFormat = "2006-01-02 15:04:05Z07:00"
)

type Config struct {
	Username string
	Password string
	UserID   int64 `toml:"user_id"`
}

type Response struct {
	Response struct {
		Message string
	}
	Data json.RawMessage
}

type User struct {
	Name    string
	Surname string
	ID      int64
}

type Event struct {
	FormatedDuration string
	ID               int64
}

type Task struct {
	Name    string
	Project string
	ID      int64
	User    User
	Users   []struct {
		ID    int64
		Event Event
	}
	Event Event
}

var config Config

func main() {
	args, err := docopt.Parse(usage, nil, true, version, false)
	if err != nil {
		panic(err)
	}

	err = ko.Load(args["--config"].(string), &config)
	if err != nil {
		log.Fatalln(err)
	}

	client := resty.New()
	client.SetBasicAuth(config.Username, config.Password)
	client.SetHostURL(BaseURL)

	switch {
	case args["start"].(bool):
		handleStart(client, args["<id>"].(string))

	case args["stop"].(bool):
		handleStop(client, args["<id>"].(string))

	case args["sync"].(bool):
		handleSync(client)

	case args["get"].(bool):
		handleGet(client)
	}
}

func handleGet(client *resty.Client) {
	task := getActiveTask(client)
	if task == nil {
		fmt.Println("no active tasks")
		return
	}

	fmt.Printf("Project: %s\n", task.Project)
	fmt.Printf("Name: %s\n", task.Name)
	fmt.Printf("ID: %v\n", task.ID)
}

func getActiveTask(client *resty.Client) *Task {
	raw, err := client.NewRequest().Get("/api/v2/tasks?filter=TRACKING")
	if err != nil {
		log.Fatalln(err)
	}

	//fmt.Println(string(raw.Body()))

	var tasks []Task
	bind(raw, &tasks)

	for _, task := range tasks {
		if task.User.ID == config.UserID {
			for _, user := range task.Users {
				if user.ID == config.UserID {
					task.Event = user.Event
					break
				}
			}

			return &task
		}
	}

	return nil
}

func getNow() string {
	date := time.Now().Format(TimeFormat)
	return date
}

func handleStart(client *resty.Client, id string) {
	query := url.Values{}
	query.Add("stop_running_task", "true")
	query.Add("date", getNow())

	raw, err := client.NewRequest().Get(
		"/api/v4/tasks/track/" + id + "?" + query.Encode(),
	)
	if err != nil {
		log.Fatalln(err)
	}

	response := bind(raw, nil)
	fmt.Println(response.Response.Message)
}

func handleStop(client *resty.Client, id string) {
	query := url.Values{}
	query.Add("date", getNow())

	raw, err := client.NewRequest().Get(
		"/api/v4/tasks/stop/" + id + "?" + query.Encode(),
	)
	if err != nil {
		log.Fatalln(err)
	}

	response := bind(raw, nil)
	fmt.Println(response.Response.Message)
}

func handleSync(client *resty.Client) {
	task := getActiveTask(client)
	if task == nil {
		fmt.Println("no active tasks")
		return
	}

	query := url.Values{}
	query.Add("event_id", fmt.Sprint(task.Event.ID))
	query.Add("date", getNow())
	query.Add("return_task", "true")

	raw, err := client.NewRequest().Get(
		"/api/v4/tasks/sync/" + fmt.Sprint(task.ID) + "?" + query.Encode(),
	)
	if err != nil {
		log.Fatalln(err)
	}

	response := bind(raw, task)
	fmt.Println(response.Response.Message)
	fmt.Println(task.Event.FormatedDuration)
}

func bind(raw *resty.Response, to interface{}) *Response {
	var response Response
	err := json.Unmarshal(raw.Body(), &response)
	if err != nil {
		log.Fatalln(err)
	}

	if to != nil {
		err = json.Unmarshal(response.Data, &to)
		if err != nil {
			log.Fatalln(err)
		}
	}

	return &response
}
