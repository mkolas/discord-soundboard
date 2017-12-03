package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
	"log"

	"database/sql"
)

var (
	// Map of Guild id's to *Play channels, used for queuing and rate-limiting guilds
	queues map[string]chan *Play = make(map[string]chan *Play)

	sounds   = []*Sound{}
	soundMap = map[string]*Sound{}

	MAX_QUEUE_SIZE = 128

	token  string
	status string

	db *sql.DB

	dg *discordgo.Session
)

// Right now, configuration only set to take in a bot token. but we can add in more things in the future.
type Configuration struct {
	Token  string
	Status string
	Port   string
}

// Play represents an individual use of the !airhorn command
type Play struct {
	GuildID   string
	ChannelID string
	UserID    string
	Sound     *Sound
}

// Sound type cribbed from airhornbot.
type Sound struct {
	Name      string   `csv:"filename"`
	Command   string   `csv:"command"`
	Played    int      `csv:"played"`
	PartDelay int      `csv:"-"`
	buffer    [][]byte `csv:"-"`
}

func main() {

	// init rand
	rand.Seed(time.Now().Unix())

	// first lets verify that we've got a token
	confFile, err := os.Open("config/conf.json")
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(confFile)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		panic(err)
	}
	token = configuration.Token
	status = configuration.Status
	if strings.Contains(token, "ADD YOUR DISCORD BOT TOKEN HERE!") {
		fmt.Println("Please set a Discord bot token in config/conf.json.")
		return
	}
	fmt.Println("Retrieved token: " + token)

	db, err = sql.Open("sqlite3", "config/sounds.db")
	if err != nil {
		log.Fatal("Unable to create database")
	}
	createStmt := `
	create table if not exists sounds(command text, file text, played int);
	`
	_, err = db.Exec(createStmt)
	if err != nil {
		log.Printf("%q, %s\n", err, createStmt)
	}

	defer db.Close()
	// lets load up our sounds

	rows, err := db.Query("select * from sounds")
	defer rows.Close()
	for rows.Next() {
		var command string
		var file string
		var played int
		err = rows.Scan(&command, &file, &played)
		if err != nil {
			log.Fatal(err)
		}
		sound := &Sound{
			Name:    file,
			Command: command,
			Played:  played,
		}
		sounds = append(sounds, sound)
	}

	for _, sound := range sounds {
		// for each sound, load the .dca into memory and store it in the Sound struct
		sound.Load()
		soundMap[sound.Command] = sound
		fmt.Println("Loaded filename", sound.Name, "loaded command", sound.Command)
	}

	// Create a new Discord session using the provided bot token.
	dg, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	dg.AddHandler(ready)
	dg.AddHandler(messageCreate)
	dg.AddHandler(guildCreate)

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}
	http.Handle("/dsb/", http.StripPrefix("/dsb/", http.FileServer(http.Dir("web"))))
	http.Handle("/create", http.HandlerFunc(handleUpload))
	http.Handle("/get", http.HandlerFunc(handleGet))
	http.Handle("/aliases", http.HandlerFunc(handleAliases))
	http.Handle("/createAlias", http.HandlerFunc(handleCreateAlias))
	http.Handle("/delete", http.HandlerFunc(handleDelete))
	http.Handle("/restart", http.HandlerFunc(handleRestart))

	// we _must_ listen and serve AFTER declaring our handlers.
	http.ListenAndServe(configuration.Port, nil)
	fmt.Println("Discord Soundboard is now running.  Press CTRL-C to exit.")
	// Simple way to keep program running until CTRL-C is pressed.
	<-make(chan struct{})
	return
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	_ = s.UpdateStatus(0, status) // set status message defined in configuration
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	var message = m.Content
	if strings.HasPrefix(message, "!") { // we can make the prefix configurable but for right now always look for !
		for _, command := range strings.Split(message[1:], "!") {
			command := strings.TrimSpace(command)
			c, err := s.State.Channel(m.ChannelID)
			if err != nil {
				// Could not find channel.
				return
			}

			// we need to have the channel available to send a message, so do this second.
			if command == "list" || command == "commands" {
				// special case for list command.
				keys := make([]string, len(soundMap))
				i := 0
				for k := range soundMap {
					keys[i] = k
					i++
				}
				sort.Strings(keys)
				// discord has a 2000 character limit on message length. we'll need to break up our list if the length gets too long
				commandList := strings.Join(keys, ", ")
				if len(commandList) > 1900 { //lowball for safety
					keyIndex := 0
					for keyIndex < len(keys) {
						outputString := ""
						for len(outputString) < 1900 && keyIndex < len(keys) {
							outputString = outputString + keys[keyIndex] + ", "
							keyIndex++
						}
						outputString = outputString[:len(outputString)-2]
						_, _ = s.ChannelMessageSend(c.ID, "**Commands**```"+outputString+"```")
					}

				} else {
					_, _ = s.ChannelMessageSend(c.ID, "**Commands**```"+strings.Join(keys, ", ")+"```")
				}
				return
			}

			// Find the guild for that channel.
			g, err := s.State.Guild(c.GuildID)
			if err != nil {
				// Could not find guild.
				return
			}

			// get audio channel to play in
			ac := getCurrentVoiceChannel(m.Author, g, s)
			if ac == nil {
				fmt.Println("Failed to find channel to play sound in")
				return
			}

			if command == "random" {
				keys := make([]string, 0, len(soundMap))
				for k := range soundMap {
					keys = append(keys, k)
				}
				command = keys[rand.Intn(len(keys))]
			}
			if command == "least" || command == "lowest" {
				lowest := 1 <<63 -1 // maxint
				for k := range soundMap {
					if soundMap[k].Played < lowest {
						command = soundMap[k].Command
					}
				}
			}

			sound, ok := soundMap[command] // look for command in our soundMap
			if ok {
				sound.Played++
				updateStmt, err := db.Prepare("update sounds set played = ? where command = ?")
				if err != nil {
					log.Println(err)
				}
				updateStmt.Exec(sound.Played, command)
				go enqueuePlay(m.Author, ac, g, sound, s)
				go s.ChannelMessageDelete(m.ChannelID, m.ID) //clean up the command afterwards
			}
		}
		return
	}
}

// This function will be called (due to AddHandler above) every time a new
// guild is joined.
func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	if event.Guild.Unavailable {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			return
		}
	}
}

// Load attempts to load an encoded sound file from disk
// DCA files are pre-computed sound files that are easy to send to Discord.
// If you would like to create your own DCA files, please use:
// https://github.com/nstafie/dca-rs
// eg: dca-rs --raw -i <input wav file> > <output file>
func (s *Sound) Load() error {
	path := "sounds/" + s.Name

	file, err := os.Open(path)

	if err != nil {
		fmt.Println("error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// read opus frame length from dca file
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}

		if err != nil {
			fmt.Println("error reading from dca file1 :", err)
			return err
		}

		// read encoded pcm from dca file
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("error reading from dca file2 :", err)
			return err
		}

		// append encoded pcm data to the buffer
		s.buffer = append(s.buffer, InBuf)
	}
}

// Prepares and enqueues a play into the ratelimit/buffer guild queue
func enqueuePlay(user *discordgo.User, channel *discordgo.Channel, guild *discordgo.Guild, sound *Sound, session *discordgo.Session) {
	play := createPlay(user, channel, guild, sound)
	if play == nil {
		return
	}

	// Check if we already have a connection to this guild
	//   yes, this isn't threadsafe, but its "OK" 99% of the time
	_, exists := queues[guild.ID]

	if exists {
		if len(queues[guild.ID]) < MAX_QUEUE_SIZE {
			queues[guild.ID] <- play
		}
	} else {
		queues[guild.ID] = make(chan *Play, MAX_QUEUE_SIZE)
		playSound(play, nil, session)
	}
}

// Prepares a play
func createPlay(user *discordgo.User, channel *discordgo.Channel, guild *discordgo.Guild, sound *Sound) *Play {

	// Create the play
	play := &Play{
		GuildID:   guild.ID,
		ChannelID: channel.ID,
		UserID:    user.ID,
		Sound:     sound,
	}

	return play
}

// Play a sound
func playSound(play *Play, vc *discordgo.VoiceConnection, session *discordgo.Session) (err error) {
	fmt.Println("playing sound " + play.Sound.Name)

	if vc == nil {
		vc, err = session.ChannelVoiceJoin(play.GuildID, play.ChannelID, false, false)
		// vc.Receive = false
		if err != nil {
			fmt.Println("Failed to retrieve voice connection. Close and retry.")
			// this occurs when the voice connection fails to close. let's close manually?
			vc.Close() // close manually
			vc, _ = session.ChannelVoiceJoin(play.GuildID, play.ChannelID, false, false)
		}
	}

	// If we need to change channels, do that now
	if vc.ChannelID != play.ChannelID {
		vc.ChangeChannel(play.ChannelID, false, false)
		time.Sleep(time.Millisecond * 125)
	}

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(time.Millisecond * 32)

	// Play the sound
	play.Sound.Play(vc)

	// If there is another song in the queue, recurse and play that
	if len(queues[play.GuildID]) > 0 {
		play := <-queues[play.GuildID]
		playSound(play, vc, session)
		return nil
	}

	// If the queue is empty, delete it
	time.Sleep(time.Millisecond * time.Duration(play.Sound.PartDelay))
	delete(queues, play.GuildID)
	vc.Disconnect()
	return nil
}

// Plays this sound over the specified VoiceConnection
func (s *Sound) Play(vc *discordgo.VoiceConnection) {
	vc.Speaking(true)
	defer vc.Speaking(false)

	for _, buff := range s.buffer {
		vc.OpusSend <- buff
	}
}

// Attempts to find the current users voice channel inside a given guild
func getCurrentVoiceChannel(user *discordgo.User, guild *discordgo.Guild, session *discordgo.Session) *discordgo.Channel {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == user.ID {
			channel, _ := session.State.Channel(vs.ChannelID)
			return channel
		}
	}
	return nil
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("web/templates/get.html.tmpl")
	err := t.Execute(w, soundMap)
	if err != nil {
		panic(err)
	}
}

func handleAliases(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("web/templates/alias.html.tmpl")
	err := t.Execute(w, soundMap)
	if err != nil {
		fmt.Println(err)
	}
}

func handleRestart(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Attempting to restart soundboard...")
	var err error

	dg.Close()
	// Create a new Discord session using the provided bot token.
	dg, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println(err)
	}
	dg.AddHandler(ready)
	dg.AddHandler(messageCreate)
	dg.AddHandler(guildCreate)
	err = dg.Open()
	if err != nil {
		fmt.Println(err)
	}
}

func handleCreateAlias(w http.ResponseWriter, r *http.Request) {
	newAlias := r.FormValue("newAlias")
	oldCommand := r.FormValue("sound")

	sound, ok := soundMap[oldCommand]
	if ok {
		alias := &Sound{
			Name:    sound.Name,
			Command: newAlias,
		}
		alias.Load()
		soundMap[newAlias] = alias
		insertStmt, err := db.Prepare("insert into sounds(command, file, played) values(?,?,?)")
		if err != nil {
			log.Fatal(err)
		}
		insertStmt.Exec(alias.Command, alias.Name, 0)
	}

}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	commandToDelete := r.FormValue("delete")
	_, ok := soundMap[commandToDelete]
	if ok {
		delete(soundMap, commandToDelete)

		deleteStmt, err := db.Prepare("delete from sounds where command = ?")
		if err != nil {
			log.Fatalln(err)
		}

		deleteStmt.Exec(commandToDelete)
	}
	w.WriteHeader(200)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	//read file from request and save to disk
	file, header, err := r.FormFile("file")

	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	defer file.Close()

	out, err := os.Create("sounds/" + header.Filename)
	if err != nil {
		fmt.Fprintf(w, "Failed to open the file for writing")
		return
	}

	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprintln(w, err)
	}

	//create dca filename
	dcaFilename := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)) + ".dca"

	dcaOut, err := os.Create("sounds/" + dcaFilename)
	if err != nil {
		panic(err)
	}

	// convert file to .dca
	cmd := exec.Command("dca-rs", "-i", "sounds/"+header.Filename, "--raw")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	writer := bufio.NewWriter(dcaOut)

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	io.Copy(writer, stdoutPipe)
	cmd.Wait()

	fmt.Println("No errors from command")
	writer.Flush()
	dcaOut.Close()

	// create sound struct, load into map
	sound := &Sound{
		Name:    dcaFilename,
		Command: r.FormValue("command"),
	}

	sound.Load()
	soundMap[sound.Command] = sound
	fmt.Println("Loaded filename", sound.Name, "loaded command", sound.Command)
	//  insert into db
	insertStmt, err := db.Prepare("insert into sounds(command, file, played) values(?,?,?)")
	if err != nil {
		log.Fatal(err)
	}

	insertStmt.Exec(r.FormValue("command"), dcaFilename, 0)

}
