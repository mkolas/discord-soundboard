# discord-soundboard
Configurable soundboard bot for discord

The goal of this project is to create an easily-configurable, self-hosted soundboard for Discord servers. Inspired by [airhornbot](https://github.com/hammerandchisel/airhornbot), the idea here is to be able to provide a list of .dca sound files and triggers through a flat file to be able to easily add new sounds as your server generates new lore.

Please note that this is my first "real" Golang project so bear with me!

TODO:
* ~~Read in and parse flat files into Go data structures~~
* ~~Set .dca files into memory~~
* ~~Configure bot to be able to respond to different triggers and lookup related sound from data structure~~
* ~~Prevent sounds from being played on top of one another (your friends will also probably try to ruin your nice things)~~
* set up configuration structure- one with discord bot token ~~and another w/ sound list~~
* add !list functionality that will output available commands
* write full installation instructions, including explaining .dca creation process
* implement web service that will take in a .wav and command, automatically convert it to dca, and add it to app's list of sounds
* clean up some elements of code, learn more about scoping and how to set discordgo object to be available as a singleton
