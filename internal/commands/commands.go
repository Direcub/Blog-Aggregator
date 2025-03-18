package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Direcub10/Blog-Aggregator/internal/RSS"
	"github.com/Direcub10/Blog-Aggregator/internal/config"
	"github.com/Direcub10/Blog-Aggregator/internal/database"
	"github.com/google/uuid"
)

type State struct {
	Db      *database.Queries
	Pointer *config.Config
}

type Command struct {
	Name string
	Args []string
}

type Commands struct {
	Handlers map[string]func(*State, Command) error
}

func (c *Commands) Register(name string, f func(*State, Command) error) {
	c.Handlers[name] = f
}

func (c *Commands) Run(s *State, cmd Command) error {
	value, exists := c.Handlers[cmd.Name]
	if !exists {
		return fmt.Errorf("command does not exist")
	}
	err := value(s, cmd)
	if err != nil {
		return err
	}
	return nil
}

func HandlerLogin(s *State, cmd Command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: %s <name>", cmd.Name)
	}
	name := cmd.Args[0]
	_, err := s.Db.GetUser(context.Background(), name)
	if err == sql.ErrNoRows {
		fmt.Println("User does not exist")
		os.Exit(1)
	} else if err != nil {
		log.Fatal(err)
	}
	s.Pointer.SetUser(name)
	fmt.Printf("Current user has been set\n")

	return nil
}

func HandlerRegister(s *State, cmd Command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("register command takes exactly one argument: the username")
	}
	name := cmd.Args[0]
	now := time.Now()
	_, err := s.Db.GetUser(context.Background(), name)
	if err == nil {
		fmt.Println("User already exists")
		os.Exit(1)
	} else if err != sql.ErrNoRows {
		log.Fatal(err)
	}
	userInfo := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
	}
	user, err := s.Db.CreateUser(context.Background(), userInfo)
	if err != nil {
		log.Fatal(err)
	}
	s.Pointer.SetUser(name)
	fmt.Printf("User name:%v created", name)
	fmt.Printf("User data: %+v\n", user)
	return nil
}

func HandlerReset(s *State, cmd Command) error {
	err := s.Db.Reset(context.Background())
	if err != nil {
		fmt.Printf("an error has occured: %v", err)
		os.Exit(1)
	}
	return nil
}

func HandlerGetUser(s *State, cmd Command) error {
	users, err := s.Db.GetUsers(context.Background())
	if err != nil {
		fmt.Printf("an error has occured: %v", err)
		os.Exit(1)
	}
	for index, name := range users {
		if users[index].Name == s.Pointer.CurrentUsername {
			fmt.Printf("* %v (current)", name.Name)
		} else {
			fmt.Printf("* %v", name.Name)
		}
	}
	return nil
}

func HandlerAgg(s *State, cmd Command) error {
	if len(cmd.Args) < 1 || len(cmd.Args) > 2 {
		return fmt.Errorf("usage: %v <time_between_reqs>", cmd.Name)
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	log.Printf("Collecting feeds every %s...", timeBetweenRequests)

	ticker := time.NewTicker(timeBetweenRequests)

	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func scrapeFeeds(s *State) {
	feed, er := s.Db.GetNextFeedToFetch((context.Background()))
	if er != nil {
		log.Println("Couldn't get next feeds to fetch", er)
		return
	}
	log.Printf("DEBUG: GetNextFeedToFetch returned Feed ID: %d, Name: %s, URL: %s", feed.ID, feed.Name, feed.Url)
	log.Println("Found a feed to fetch!")
	scrapeFeed(s.Db, feed)
}

func scrapeFeed(db *database.Queries, feed database.Feed) {
	_, err := db.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		log.Printf("Couldn't mark feed %s fetched: %v", feed.Name, err)
		return
	}

	feedData, err := RSS.FetchFeed(context.Background(), feed.Url)
	if err != nil {
		log.Printf("Couldn't collect feed %s: %v", feed.Name, err)
		return
	}
	for _, item := range feedData.Channel.Item {
		publishedAt := sql.NullTime{}
		if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
			publishedAt = sql.NullTime{
				Time:  t,
				Valid: true,
			}
		}
		now := time.Now()
		_, err = db.AddPost(context.Background(), database.AddPostParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			FeedID:    feed.ID,
			Title:     item.Title,
			Description: sql.NullString{
				String: item.Description,
				Valid:  true,
			},
			Url:         item.Link,
			PublishedAt: publishedAt,
		})
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				continue
			}
			log.Printf("Couldn't create post: %v", err)
			continue
		}
	}
	log.Printf("Feed %s collected, %v posts found", feed.Name, len(feedData.Channel.Item))
}

func HandlerAddFeed(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 2 {
		return errors.New("usage: addfeed <name> <url>")
	}

	name := cmd.Args[0]
	url := cmd.Args[1]
	now := time.Now()
	user, err := s.Db.GetUser(context.Background(), s.Pointer.CurrentUsername)
	if err != nil {
		return err
	}

	params := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
		Url:       url,
		UserID:    user.ID,
	}

	feed, err := s.Db.CreateFeed(context.Background(), params)
	if err != nil {
		return err
	}
	fmt.Println(feed)

	followCmd := Command{
		Name: "follow",
		Args: []string{params.Url},
	}

	er := HandlerFollow(s, followCmd, user)
	if er != nil {
		return fmt.Errorf("feed added but could not follow it: %w", er)
	}
	return nil
}

func HandlerGetFeeds(s *State, cmd Command) error {
	feeds, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return err
	}
	for i := 0; i < len(feeds); i++ {
		fmt.Println(feeds[i].Name)
		fmt.Println(feeds[i].Url)
		fmt.Println(feeds[i].Name_2)
	}
	return nil
}

func HandlerFollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return errors.New("usage: follow <url>")
	}

	url := cmd.Args[0]
	now := time.Now()
	currentUser, err := s.Db.GetUser(context.Background(), s.Pointer.CurrentUsername)
	if err != nil {
		return err
	}

	feed, err := s.Db.GetFeedByURL(context.Background(), url)
	if err != nil {
		return err
	}
	feedID := feed.ID

	params := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    currentUser.ID,
		FeedID:    feedID,
	}
	row, err := s.Db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return err
	}
	fmt.Printf("%s has followed %s", row.UserName, row.FeedName)
	return nil
}

func HandlerGetFollows(s *State, cmd Command, user database.User) error {
	currentUser, err := s.Db.GetUser(context.Background(), s.Pointer.CurrentUsername)
	if err != nil {
		return err
	}

	feeds, err := s.Db.GetFeedFollowsForUser(context.Background(), currentUser.ID)
	if err != nil {
		return err
	}

	if len(feeds) == 0 {
		fmt.Println("You're not following any feeds")
		return nil
	}

	fmt.Println("You are following:")
	for i := 0; i < len(feeds); i++ {
		fmt.Printf("-%s\n", feeds[i].FeedName)
	}
	return nil
}

func HandlerUnFollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: %s <feed_url>", cmd.Name)
	}

	feed, err := s.Db.GetFeedByURL(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("couldn't get feed: %w", err)
	}

	params := database.RemoveFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}
	err = s.Db.RemoveFollow(context.Background(), params)
	if err != nil {
		return err
	}
	fmt.Printf("%s unfollowed successfully!\n", feed.Name)
	return nil
}

func HandlerBrowse(s *State, cmd Command, user database.User) error {
	limit := 2
	if len(cmd.Args) == 1 {
		if specifiedLimit, err := strconv.Atoi(cmd.Args[0]); err == nil {
			limit = specifiedLimit
		} else {
			return fmt.Errorf("invalid limit: %w", err)
		}
	}

	posts, err := s.Db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	})
	if err != nil {
		return fmt.Errorf("couldn't get posts for user: %w", err)
	}

	fmt.Printf("Found %d posts for user %s:\n", len(posts), user.Name)
	for _, post := range posts {
		fmt.Printf("%s from %s\n", post.PublishedAt.Time.Format("Mon Jan 2"), post.FeedName)
		fmt.Printf("--- %s ---\n", post.Title)
		fmt.Printf("    %v\n", post.Description.String)
		fmt.Printf("Link: %s\n", post.Url)
		fmt.Println("=====================================")
	}

	return nil
}
