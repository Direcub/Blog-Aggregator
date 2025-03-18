package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/Direcub10/Blog-Aggregator/internal/commands"
	"github.com/Direcub10/Blog-Aggregator/internal/config"
	"github.com/Direcub10/Blog-Aggregator/internal/database"
	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		fmt.Printf("Error reading file: %v", err)
		os.Exit(1)
	}

	dbURL := cfg.DBURL
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)
	state := commands.State{
		Pointer: &cfg,
		Db:      dbQueries,
	}

	cmd := commands.Commands{
		Handlers: make(map[string]func(*commands.State, commands.Command) error),
	}

	cmd.Register("login", commands.HandlerLogin)
	cmd.Register("register", commands.HandlerRegister)
	cmd.Register("reset", commands.HandlerReset)
	cmd.Register("users", commands.HandlerGetUser)
	cmd.Register("agg", commands.HandlerAgg)
	cmd.Register("addfeed", middlewareLoggedIn(commands.HandlerAddFeed))
	cmd.Register("feeds", commands.HandlerGetFeeds)
	cmd.Register("follow", middlewareLoggedIn(commands.HandlerFollow))
	cmd.Register("following", middlewareLoggedIn(commands.HandlerGetFollows))
	cmd.Register("unfollow", middlewareLoggedIn(commands.HandlerUnFollow))
	cmd.Register("browse", middlewareLoggedIn(commands.HandlerBrowse))

	args := os.Args
	if len(args) < 2 {
		fmt.Println("too few arguements")
		os.Exit(1)
	}

	commandName := args[1]
	arguements := args[2:]
	newcmd := commands.Command{
		Name: commandName,
		Args: arguements,
	}
	error := cmd.Run(&state, newcmd)
	if error != nil {
		fmt.Printf("error occurred: %v\n", error)
		os.Exit(1)
	}
}

func middlewareLoggedIn(handler func(s *commands.State, cmd commands.Command, user database.User) error) func(*commands.State, commands.Command) error {
	return func(s *commands.State, cmd commands.Command) error {
		user, err := s.Db.GetUser(context.Background(), s.Pointer.CurrentUsername)
		if err != nil {
			return err
		}

		return handler(s, cmd, user)
	}
}
