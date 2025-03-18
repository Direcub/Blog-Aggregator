# Blog-Aggregator
This program relies on postgres and Go to find, track, and store configured rss feeds
to work, this program will need access to a database named "gator" (made by user with postgres)
after that, find the connection string for that database and place it in your home directory in a json file names .gatorconfig.json
run go build and the aggregator should be working, commands (listed below) can be used like "./Blog-Aggregator register dire"
Commands:
    Login: Sets current user 
    Register: adds a new user to the database
    Reset: DEBUG ONLY, clears database of users
    Users: list all users on local database
    Agg: begins aggregating rss feeds
    AddFeed: adds a feed to track, takes a url
    Follow: follows a previously existing feed in the databse
    Following: shows all feeds the user is currently following
    Unfollow: yes
    Browses: shows aggregated post, optionally takes an Integer to show more post