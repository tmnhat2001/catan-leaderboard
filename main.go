package main

import (
  "github.com/bwmarrin/discordgo"
  "github.com/jackc/pgx/v4"
  "context"
  "fmt"
  "os"
  "os/signal"
  "syscall"
  "strings"
)

const COMMAND_PREFIX = "catan!"

var db_conn *pgx.Conn

func init() {
  var err error
  db_conn, err = pgx.Connect(context.Background(), os.Getenv("POSTGRESQL_URL"))
  if err != nil {
    fmt.Println("Error connecting to the database: ", err)
    os.Exit(1)
  }
}

func main() {
  defer db_conn.Close(context.Background())

  token := os.Getenv("BOT_TOKEN")
  discord, err := discordgo.New("Bot " + token)
  if err != nil {
    fmt.Println("Error creating Discord session: ", err)
    os.Exit(1)
  }

  discord.AddHandler(messageCreate)

  err = discord.Open()
  if err != nil {
    fmt.Println("Error opening connection: ", err)
    os.Exit(1)
  }

  fmt.Println("Bot is now running. Press CTRL-C to exit.")
  sc := make(chan os.Signal, 1)
  signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
  <-sc

  discord.Close()
}

func messageCreate(session *discordgo.Session, m *discordgo.MessageCreate) {
  if m.Author.ID == session.State.User.ID {
    return
  }

  if strings.HasPrefix(m.Content, COMMAND_PREFIX) {
    message := strings.Split(m.Content, " ")
    if message[1] == "adduser" {
      addUserCommand(session, m, message)
    } else if message[1] == "addwin" {
      addWinCommand(session, m, message)
    } else if message[1] == "leaderboard" {
      showLeaderboardCommand(session, m, message)
    }
  }
}

func addUserCommand(session *discordgo.Session, m *discordgo.MessageCreate, message []string) {
  if len(message) == 3 {
    _, err := db_conn.Exec(context.Background(), "INSERT INTO users (username, guild_id) VALUES ($1, $2)", message[2], m.GuildID)
    if err != nil {
      session.ChannelMessageSend(m.ChannelID, "An error has occurred")
      fmt.Println("Error: ", err)
      return
    }


    response := fmt.Sprintf("Successfully added user: %s", message[2])
    session.ChannelMessageSend(m.ChannelID, response)
  } else {
    session.ChannelMessageSend(m.ChannelID, "Command format: adduser [username]")
  }
}

func addWinCommand(session *discordgo.Session, m *discordgo.MessageCreate, message []string) {
  if len(message) == 3 {
    row := db_conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE username = ($1) AND guild_id = ($2);", message[2], m.GuildID)
    var record_exists int
    err := row.Scan(&record_exists)
    if err != nil {
      session.ChannelMessageSend(m.ChannelID, "An error has occurred")
      fmt.Println("Error: ", err)
      return
    }

    var response string
    if record_exists == 0 {
      response = fmt.Sprintf("User %s does not exist", message[2])
    } else {
      _, err = db_conn.Exec(context.Background(), "UPDATE users SET games_won = games_won + 1 WHERE username = ($1) AND guild_id = ($2)", message[2], m.GuildID)
      if err != nil {
        session.ChannelMessageSend(m.ChannelID, "An error has occurred")
        fmt.Println("Error: ", err)
        return
      }

      response = fmt.Sprintf("Congrats %s on the win!", message[2])
    }

    session.ChannelMessageSend(m.ChannelID, response)
  } else {
    session.ChannelMessageSend(m.ChannelID, "Command format: addwin [username]")
  }
}

func showLeaderboardCommand(session *discordgo.Session, m *discordgo.MessageCreate, message []string) {
  rows, err := db_conn.Query(context.Background(), "SELECT RANK() OVER (ORDER BY games_won DESC), username, games_won FROM users LIMIT 5")
  if err != nil {
    session.ChannelMessageSend(m.ChannelID, "An error has occurred")
    fmt.Println("Error: ", err)
    return
  }

  defer rows.Close()

  var response_rows [5]string

  for rows.Next() {
    var rank int
    var username string
    var games_won int
    err = rows.Scan(&rank, &username, &games_won)
    if err != nil {
      session.ChannelMessageSend(m.ChannelID, "An error has occurred")
      fmt.Println("Error: ", err)
      return
    }

    response_row := fmt.Sprintf("%d. %s - %d games won", rank, username, games_won)
    response_rows[rank-1] = response_row
  }

  if rows.Err() != nil {
    session.ChannelMessageSend(m.ChannelID, "An error has occurred")
    fmt.Println("Error: ", err)
    return
  }

  response := strings.Join(response_rows[:], "\n")
  session.ChannelMessageSend(m.ChannelID, response)
}
