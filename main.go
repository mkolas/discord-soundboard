package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"encoding/csv"
	"github.com/bwmarrin/discordgo"
)

func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

var token string

var (
	// discordgo session
	discord *discordgo.Session

	// Map of Guild id's to *Play channels, used for queuing and rate-limiting guilds
	queues map[string]chan *Play = make(map[string]chan *Play)

	sounds = []*Sound{}

	soundMap = map[string]*Sound{}

	// Sound encoding settings
	BITRATE        = 128
	MAX_QUEUE_SIZE = 6

	// Owner
	OWNER string
)

// Play represents an individual use of the !airhorn command
type Play struct {
	GuildID   string
	ChannelID string
	UserID    string
	Sound     *Sound
}


// Sound type cribbed from airhornbot.
type Sound struct {
	Name string `csv:"filename"`

	// major difference here is that we want to be able to call each sound explicitly
	Command string `csv:"command"`

	// Really not sure how important this is. let's defa
	PartDelay int `csv:"-"`

	// Buffer to store encoded PCM packets
	buffer [][]byte `csv:"-"`
}

func main() {

	// Instead, let's pull a token from a configuration file.
	if token == "" {
		fmt.Println("No token provided. Please run: buster -t <bot token>")
		return
	}

	// cribbed from goscv example
	soundsFile, err := os.OpenFile("config/sounds.csv", os.O_RDWR|os.O_CREATE, os.ModePerm) // should figure out what these os objects are
	if err != nil {
		panic(err)
	}
	defer soundsFile.Close()

	reader := csv.NewReader(soundsFile)
	//Configure reader options Ref http://golang.org/src/pkg/encoding/csv/reader.go?s=#L81
	reader.Comma = ','          //field delimiter
	reader.Comment = '#'        //Comment character
	reader.FieldsPerRecord = 2 //Number of records per record. Set to Negative value for variable
	reader.TrimLeadingSpace = true

	for {
		// read just one record, but we could ReadAll() as well
		record, err := reader.Read()
		// end-of-file is fitted into err
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("Error:", err)
			reader.Read()
			continue
		}
		// record is array of strings Ref http://golang.org/src/pkg/encoding/csv/reader.go?s=#L134
			// Create the play
		sound := &Sound{
			Name:   record[0],
			Command: record[1],
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
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// Register ready as a callback for the ready events.
	dg.AddHandler(ready)

	// Register messageCreate as a callback for the messageCreate events.
	dg.AddHandler(messageCreate)

	// Register guildCreate as a callback for the guildCreate events.
	dg.AddHandler(guildCreate)

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}

	fmt.Println("Bobby is now running.  Press CTRL-C to exit.")
	// Simple way to keep program running until CTRL-C is pressed.
	<-make(chan struct{})
	return
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	// Set the playing status.
	_ = s.UpdateStatus(0, "!bobby")
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if strings.HasPrefix(m.Content, "!") { // we can make the prefix configurable but for right now always look for !
		command := m.Content[1:] //substring starting at index 1

		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			// Could not find channel.
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

		i, ok := soundMap[command] // look for command in our soundMap
		if ok { // we found it, so lets queue the sound
			go enqueuePlay(m.Author, ac, g, i, s)
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
			_, _ = s.ChannelMessageSend(channel.ID, "Bobby Moynihan is ready! Type !bobby while in a voice channel to play a sound.")
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
	path := "sounds/"+s.Name

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
			fmt.Println("error reading from dca file :", err)
			return err
		}

		// read encoded pcm from dca file
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("error reading from dca file :", err)
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
	fmt.Println("playing sound"+play.Sound.Name)

	if vc == nil {
		vc, err = session.ChannelVoiceJoin(play.GuildID, play.ChannelID, false, false)
		// vc.Receive = false
		if err != nil {
			fmt.Println("Failed to play sound")
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