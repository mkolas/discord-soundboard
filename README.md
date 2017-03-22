# discord-soundboard
Configurable soundboard for Discord

The goal of this project is to create an easily-configurable, self-hosted soundboard for Discord communities. Inspired by [airhornbot](https://github.com/hammerandchisel/airhornbot), my community wanted a similar bot tailored to our personal in-jokes and lore. This project uses much of the same data structures and Discord API calls as airhornbot, but is less focused on organizing sounds into collections and more focused on providing a growable list of specific commands.

Please note that this is my first "real" project in Golang so bear with me! :)

TODO:
* Work on my Golang finesse- learn more about scoping and how to set discordgo object to be available as a singleton object
* Rework persistence mechanism from a flat file to something like redis. If the primary interace for adding sounds will be through HTTP, then a CSV isn't the best solution
* Track stats on sound usage (should be easy with redis)
* Serve HTML form from within Go- don't just have it tossed onto an apache2 /var/www
* Improve process of inviting bot to server (how does Discord Music Bot Work?)

## Installation Instructions

### Set up a Discord Application

Before you pull or run any code, browse to your [Discord My Apps](https://discordapp.com/developers/applications/me) page and create a new app. The name and icon don't matter- just choose something fun that your community will appreciate. Once your app is created, you'll need to click the **Create a Bot User** button. Once you've done that, you should be able to access your Token, which you'll need to plug into Discord Soundboard to use.

You also need to invite your bot to your server, which you can do right now. Take the following URL, and replace the client ID with the client ID listed on your application's page:

`https://discordapp.com/oauth2/authorize?&client_id=YOUR_CLIENT_ID_HERE&scope=bot&permissions=0 `

You should be directed to a standardized Discord authorization prompt, which will allow you to add your bot to your server, provided you have the correct permissions.

### Install Go

I'm not going to get into the weeds of installing Go on your system, but following [Golang's getting started](https://golang.org/doc/install) page should get you 99% of the way there. Just make sure that you set your `GOPATH` environment variable so that `go get` and `go install` work correctly.

### Pull From Github

Once you have Go installed and ready to "go" (:wink:), pull down this project with `go get github.com/mkolas/discord-soundboard`.

### Configure

In `discord-soundboard/config/config.json`, replace the Token value from the existing dummy string with the bot token provided by the Discord My Apps page.

### Run!

The bot can then be run with `go run main.go` from the `$GOPATH/src/github.com/mkolas/discord-soundboard` directory. More preferably, you can run `go install` which will create a `discord-soundboard` execuable in `$GOPATH/bin`. You'll have to copy the `config` and `sounds` directories over to `$GOPATH/bin`, but running from the `bin` directory will allow you to keep an installed binary away from any code you may choose to modify. 

### Commands

Typing `!commands` in your Discord chat should cause the Soundboard to output the list of available commands in the system. Each command should be prefixed with a `!`. This prefix character will be configuable eventually.

### Creating Sounds

Sounds can be added to the Soundboard in one of two ways:

1. Using [dca-rs](https://github.com/nstafie/dca-rs), you can generate .dca files and then add them to `config/sounds.csv`. Each row should identify the location of the sound (from `sounds/`) and the command (without `!` prefix) used to play it. Using `dca-rs`, the command to generate a sound should be `./dca-rs -i <input wav file> --raw > <output file>`. Make sure that you don't forget the `--raw` flag!
2. In the web directory, I have a simple HTML form that you can shove into a served web directory that will make a simple call to an HTTP endpoint set up by the bot. This process will automagically generate a .dca file from a .wav (or .mp3, usually) and add it to the list of available sounds. Just make sure that `dca-rs` and `ffmpeg` are available on your `PATH`, otherwise this might not work. In addition, this service is far from battle tested and probably insecure as heck. Use with caution. If you'd to make this available for others to use, make sure to point the form action to the server hosting the bot and form.


### Questions, Comments, Concerns?

Please feel free to get at me- I'd love more folks to be able to experience this wonderful and weird project.
