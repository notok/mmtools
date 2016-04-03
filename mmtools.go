package main

import (
	"flag"
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/notok/mmtools/lib/platform/model"
	"os"
	"strings"
	"time"
)

// debuglog functions
type debugT bool

func (d debugT) Printf(format string, args ...interface{}) {
	if d {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

func (d debugT) PrintVar(args ...interface{}) {
	if d {
		pp.Fprintln(os.Stderr, args...)
	}
}

func (d debugT) PrintSep(str string) {
	max := 80
	if d {
		e := len(str) % 2
		n := (max - len(str) - e - 2) / 2
		fmt.Fprintln(os.Stderr, strings.Repeat("=", n), str, strings.Repeat("=", n+e))
	}
}

var debug = debugT(os.Getenv("MMDEBUG") == "TRUE")

// mattermost functions
type MMInfo struct {
	ep   string
	team string
	mail string
	pass string
}

func GetMMInfo() *MMInfo {
	return &MMInfo{os.Getenv("MMEP"), os.Getenv("MMTEAM"), os.Getenv("MMMAIL"), os.Getenv("MMPASS")}
}

func HandleError(err *model.AppError) {
	debug.PrintVar(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

type MMClient struct {
	Client   *model.Client
	UserInfo *model.User
}

func CheckMMInfo(info *MMInfo) {
	key := ""
	switch "" {
	case info.ep:
		key = "MMEP"
	case info.team:
		key = "MMTEAM"
	case info.mail:
		key = "MMMAIL"
	case info.pass:
		key = "MMPASS"
	default:
		return
	}
	fmt.Fprintln(os.Stderr, "Environment variable is not set:", key)
	os.Exit(1)
}

func GetMMClient() *MMClient {
	mminfo := GetMMInfo()
	debug.PrintVar(mminfo)
	CheckMMInfo(mminfo)
	c := model.NewClient(mminfo.ep)
	var user *model.User
	debug.PrintSep("login")
	if res, err := c.LoginByEmail(mminfo.team, mminfo.mail, mminfo.pass); err != nil {
		HandleError(err)
	} else {
		debug.PrintVar(res)
		user = res.Data.(*model.User)
	}
	return &MMClient{c, user}
}

func PrintPosts(order []string, posts map[string]*model.Post) {
	for i, postid := range order {
		debug.PrintSep(fmt.Sprintf("post %v %v", i, postid))
		post := posts[postid]
		PrintPost(post)
	}
}

func PrintPost(post *model.Post) {
	debug.PrintVar(post)
	fmt.Println(time.Unix(post.CreateAt/1000, 0))
}

// command functions
func usage() {
	fmt.Fprintln(os.Stderr, `usage: mmtools <command> [<args>]
Available commands are:
    ListUsers   List users
    ExportChannel     Export specified channel`)
}

func main() {
	// Option definitions
	listUsersCommand := flag.NewFlagSet("ListUsers", flag.ExitOnError)
	onlineFlag := listUsersCommand.Bool("online", false, "Online users.")
	offlineFlag := listUsersCommand.Bool("offline", false, "Offline users.")

	exportChannelCommand := flag.NewFlagSet("ExportChannel", flag.ExitOnError)
	channelFlag := exportChannelCommand.String("name", "", "Name of channel to export.")

	// Check subcommands
	if len(os.Args) == 1 {
		usage()
		return
	}

	// Parse subcommand options
	switch os.Args[1] {
	case "ListUsers":
		listUsersCommand.Parse(os.Args[2:])
	case "ExportChannel":
		exportChannelCommand.Parse(os.Args[2:])
	default:
		fmt.Printf("%q is not valid command.\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	// Execute subcommands
	if listUsersCommand.Parsed() {
		debug.PrintSep("ListUsers")
		mmClient := GetMMClient()
		debug.PrintSep("UserProfile")
		var users map[string]*model.User
		if res, err := mmClient.Client.GetProfiles(mmClient.UserInfo.TeamId, ""); err != nil {
			HandleError(err)
		} else {
			debug.PrintVar(res)
			users = res.Data.(map[string]*model.User)
			debug.PrintSep("UserProfile")
			debug.PrintVar(users)
		}
		debug.PrintSep("UserStatus")
		keys := make([]string, 0, len(users))
		for k := range users {
			keys = append(keys, k)
		}
		if res, err := mmClient.Client.GetStatuses(keys); err != nil {
			HandleError(err)
		} else {
			debug.PrintVar(res)
			statuses := res.Data.(map[string]string)
			switch {
			case *onlineFlag:
				debug.PrintSep("Printing Online Users")
				for userid, status := range statuses {
					if status == "online" {
						fmt.Println(users[userid].Username)
					}
				}
			case *offlineFlag:
				debug.PrintSep("Printing Offline Users")
				for userid, status := range statuses {
					if status == "offline" {
						fmt.Println(users[userid].Username)
					}
				}
			default:
				debug.PrintSep("Printing All Users with status")
				for userid, status := range statuses {
					fmt.Println(users[userid].Username, status)
				}
			}
		}
	}

	if exportChannelCommand.Parsed() {
		if *channelFlag == "" {
			fmt.Println("Please supply the recipient using -recipient option.")
			return
		}
		mmClient := GetMMClient()
		debug.PrintSep("get channel")
		var channels []*model.Channel
		if res, err := mmClient.Client.GetChannels(""); err != nil {
			HandleError(err)
		} else {
			debug.PrintVar(res)
			channels = res.Data.(*model.ChannelList).Channels
			members := res.Data.(*model.ChannelList).Members
			_ = members
			debug.PrintSep("channels")
			debug.PrintVar(channels)
		}
		// TODO user list is required to identify who sent the post.
		for i, channel := range channels {
			debug.PrintSep(fmt.Sprintf("channel %v", i))
			debug.PrintVar(channel)
			if channel.Name == "town-square" {
				continue
			}
			debug.PrintSep("getPosts")
			if res, err := mmClient.Client.GetPostsSince(channel.Id, 0); err != nil {
				HandleError(err)
			} else {
				debug.PrintVar(res)
				order := res.Data.(*model.PostList).Order
				posts := res.Data.(*model.PostList).Posts
				PrintPosts(order, posts)
			}
		}
	}
	debug.PrintSep("END")
}
